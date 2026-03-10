package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebSearchTool searches the web using Brave Search API or DuckDuckGo
type WebSearchTool struct {
	provider   string // "brave" or "duckduckgo"
	apiKey     string
	maxResults int
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool(provider, apiKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 5
	}
	if provider == "" {
		provider = "brave" // Default to Brave
	}
	return &WebSearchTool{
		provider:   provider,
		apiKey:     apiKey,
		maxResults: maxResults,
	}
}

// Name returns the tool name
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Description returns the tool description
func (t *WebSearchTool) Description() string {
	return "Search the web. Returns titles, URLs, and snippets."
}

// Parameters returns the JSON schema for parameters
func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query"
			},
			"count": {
				"type": "integer",
				"description": "Results count (1-10)"
			}
		},
		"required": ["query"]
	}`)
}

// ConcurrentSafe returns true - web search is stateless and safe to run concurrently
func (t *WebSearchTool) ConcurrentSafe() bool {
	return true
}

// Execute performs the web search
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "Error: query is required", nil
	}

	count := t.maxResults
	if c, ok := args["count"].(float64); ok {
		n := int(c)
		if n < 1 {
			n = 1
		} else if n > 10 {
			n = 10
		}
		count = n
	}

	// Route to appropriate search provider based on configuration
	if t.provider == "duckduckgo" {
		return t.executeDuckDuckGoSearch(ctx, query, count)
	}

	// Default to Brave search
	return t.executeBraveSearch(ctx, query, count)
}

// executeBraveSearch performs search using Brave API
func (t *WebSearchTool) executeBraveSearch(ctx context.Context, query string, count int) (string, error) {
	if t.apiKey == "" {
		t.apiKey = os.Getenv("BRAVE_API_KEY")
	}

	if t.apiKey == "" {
		return "Error: BRAVE_API_KEY not configured", nil
	}

	// Make request to Brave Search API
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.search.brave.com/res/v1/web/search", nil)
	if err != nil {
		return "Error: " + err.Error(), nil
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", count))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Error: search API returned status %d", resp.StatusCode), nil
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Error: " + err.Error(), nil
	}

	webResults, ok := result["web"].(map[string]interface{})
	if !ok {
		return fmt.Sprintf("No results for: %s", query), nil
	}

	results, ok := webResults["results"].([]interface{})
	if !ok || len(results) == 0 {
		return fmt.Sprintf("No results for: %s", query), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Results for: %s\n", query))

	for i, r := range results {
		item := r.(map[string]interface{})
		title := getMapString(item, "title")
		itemURL := getMapString(item, "url")
		desc := getMapString(item, "description")

		lines = append(lines, fmt.Sprintf("%d. %s\n   %s", i+1, title, itemURL))
		if desc != "" {
			lines = append(lines, fmt.Sprintf("   %s", desc))
		}
	}

	return strings.Join(lines, "\n"), nil
}

// executeDuckDuckGoSearch performs search using DuckDuckGo
func (t *WebSearchTool) executeDuckDuckGoSearch(ctx context.Context, query string, count int) (string, error) {
	// DuckDuckGo Instant Answer API
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=0", url.QueryEscape(query))

	// Retry logic for DuckDuckGo API (can be slow)
	maxRetries := 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		client := &http.Client{Timeout: 30 * time.Second}
		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return "Error: " + err.Error(), nil
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "Error: " + err.Error(), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("Error: DuckDuckGo API returned status %d", resp.StatusCode), nil
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return "Error: " + err.Error(), nil
		}

		// Success - parse and return results
		var lines []string
		lines = append(lines, fmt.Sprintf("Results for: %s\n", query))

		usedCount := 0

		// Process abstract (instant answer) - doesn't count towards result limit
		if abstract, ok := result["Abstract"].(string); ok && abstract != "" {
			lines = append(lines, fmt.Sprintf("Abstract: %s\n", abstract))
		}

		// Process related topics
		if relatedTopics, ok := result["RelatedTopics"].([]interface{}); ok && len(relatedTopics) > 0 {
			lines = append(lines, "\nRelated Topics:")
			for i, topic := range relatedTopics {
				if usedCount >= count {
					break
				}
				topicMap, ok := topic.(map[string]interface{})
				if !ok {
					continue
				}
				text := getMapString(topicMap, "Text")
				firstURL := getMapString(topicMap, "FirstURL")
				if text != "" {
					lines = append(lines, fmt.Sprintf("%d. %s\n   %s", i+1, text, firstURL))
					usedCount++
				}
			}
		}

		// Process results (only if we haven't reached the count limit)
		if usedCount < count {
			if results, ok := result["Results"].([]interface{}); ok && len(results) > 0 {
				lines = append(lines, "\nResults:")
				for i, r := range results {
					if usedCount >= count {
						break
					}
					resultMap, ok := r.(map[string]interface{})
					if !ok {
						continue
					}
					text := getMapString(resultMap, "Text")
					resultURL := getMapString(resultMap, "FirstURL")
					if text != "" {
						lines = append(lines, fmt.Sprintf("%d. %s\n   %s", i+1, text, resultURL))
						usedCount++
					}
				}
			}
		}

		if len(lines) == 1 {
			return fmt.Sprintf("No results for: %s", query), nil
		}

		return strings.Join(lines, "\n"), nil
	}

	return "Error: DuckDuckGo API timeout after retries", nil
}

// WebFetchTool fetches and extracts content from URLs
type WebFetchTool struct {
	maxChars int
}

// NewWebFetchTool creates a new web fetch tool
func NewWebFetchTool(maxChars int) *WebFetchTool {
	if maxChars <= 0 {
		maxChars = 50000
	}
	return &WebFetchTool{maxChars: maxChars}
}

// Name returns the tool name
func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

// Description returns the tool description
func (t *WebFetchTool) Description() string {
	return "Fetch URL and extract readable content (HTML → markdown/text)."
}

// Parameters returns the JSON schema for parameters
func (t *WebFetchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL to fetch"
			},
			"extractMode": {
				"type": "string",
				"enum": ["markdown", "text"],
				"description": "Extraction mode"
			},
			"maxChars": {
				"type": "integer",
				"description": "Maximum characters to return"
			}
		},
		"required": ["url"]
	}`)
}

// Execute fetches the URL
func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return formatFetchError("URL validation failed", urlStr), nil
	}

	// Validate URL
	if err := validateURL(urlStr); err != nil {
		return formatFetchError(err.Error(), urlStr), nil
	}

	extractMode := "markdown"
	if mode, ok := args["extractMode"].(string); ok && mode == "text" {
		extractMode = "text"
	}

	maxChars := t.maxChars
	if mc, ok := args["maxChars"].(float64); ok {
		n := int(mc)
		if n < 100 {
			n = 100
		}
		maxChars = n
	}

	// Fetch the URL
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return formatFetchError(err.Error(), urlStr), nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return formatFetchError(err.Error(), urlStr), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return formatFetchError(fmt.Sprintf("status %d", resp.StatusCode), urlStr), nil
	}

	contentType := resp.Header.Get("content-type")
	var text, extractor string
	var truncated bool

	if strings.Contains(contentType, "application/json") {
		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			text = string(jsonBytes)
			extractor = "json"
		} else {
			buf := make([]byte, 4096)
			n, _ := resp.Body.Read(buf)
			text = string(buf[:n])
			extractor = "raw"
		}
	} else if strings.Contains(contentType, "text/html") || strings.HasPrefix(strings.ToLower(resp.Header.Get("content-type")), "text/html") {
		// Read HTML content
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return formatFetchError(err.Error(), urlStr), nil
		}

		doc, err := html.Parse(strings.NewReader(string(content)))
		if err != nil {
			text = string(content)
			extractor = "raw"
		} else {
			title := extractTitle(doc)
			content := extractBodyContent(doc)

			if extractMode == "markdown" {
				text = convertToMarkdown(title, content)
			} else {
				text = stripTags(content)
			}

			if title != "" {
				text = "# " + title + "\n\n" + text
			}
			extractor = "readability"
		}
	} else {
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return formatFetchError(err.Error(), urlStr), nil
		}
		text = string(content)
		extractor = "raw"
	}

	if len(text) > maxChars {
		truncated = true
		text = text[:maxChars]
	}

	result := map[string]interface{}{
		"url":       urlStr,
		"finalUrl":  resp.Request.URL.String(),
		"status":    resp.StatusCode,
		"extractor": extractor,
		"truncated": truncated,
		"length":    len(text),
		"text":      text,
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// ConcurrentSafe returns true - web fetch is stateless and safe to run concurrently
func (t *WebFetchTool) ConcurrentSafe() bool {
	return true
}

// Helper functions
func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func validateURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", err.Error())
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed, got '%s'", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing domain")
	}
	return nil
}

func formatFetchError(errMsg, urlStr string) string {
	result := map[string]interface{}{
		"error": errMsg,
		"url":   urlStr,
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult)
}

func extractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild != nil {
			return n.FirstChild.Data
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title := extractTitle(c); title != "" {
			return title
		}
	}
	return ""
}

func extractBodyContent(n *html.Node) string {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "nav", "header", "footer", "aside":
			return ""
		}
	}

	var content strings.Builder
	extractText(n, &content)
	return content.String()
}

func extractText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
				sb.WriteString(" ")
			}
			sb.WriteString(text)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
		if c.NextSibling != nil {
			switch c.Type {
			case html.ElementNode:
				sb.WriteString("\n\n")
			case html.TextNode:
				if next := c.NextSibling; next != nil && next.Type == html.ElementNode {
					sb.WriteString("\n")
				}
			}
		}
	}
}

func stripTags(htmlStr string) string {
	re := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStr = re.ReplaceAllString(htmlStr, "")
	re = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlStr = re.ReplaceAllString(htmlStr, "")
	re = regexp.MustCompile(`<[^>]+>`)
	htmlStr = re.ReplaceAllString(htmlStr, "")

	// Decode HTML entities
	result := strings.ReplaceAll(htmlStr, "&nbsp;", " ")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	result = strings.ReplaceAll(result, "&#39;", "'")

	// Normalize whitespace
	re = regexp.MustCompile(`[ \t]+`)
	result = re.ReplaceAllString(result, " ")
	re = regexp.MustCompile(`\n{3,}`)
	result = re.ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

func convertToMarkdown(title, content string) string {
	// Convert links
	linkRe := regexp.MustCompile(`(?i)<a\s+[^>]*href=["']([^"']+)["'][^>]*>([^<]*?)</a>`)
	content = linkRe.ReplaceAllStringFunc(content, func(match string) string {
		matches := linkRe.FindStringSubmatch(match)
		if len(matches) >= 3 {
			text := strings.TrimSpace(matches[2])
			href := matches[1]
			return fmt.Sprintf("[%s](%s)", text, href)
		}
		return match
	})

	// Convert headings
	for i := 6; i >= 1; i-- {
		re := regexp.MustCompile(fmt.Sprintf(`(?i)<h%d[^>]*>([^<]*?)</h%d>`, i, i))
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			matches := re.FindStringSubmatch(match)
			if len(matches) >= 2 {
				text := stripTags(matches[1])
				return fmt.Sprintf("\n%s %s\n", strings.Repeat("#", i), text)
			}
			return match
		})
	}

	// Convert lists
	liRe := regexp.MustCompile(`(?i)<li[^>]*>([^<]*?)</li>`)
	content = liRe.ReplaceAllStringFunc(content, func(match string) string {
		matches := liRe.FindStringSubmatch(match)
		if len(matches) >= 2 {
			text := stripTags(matches[1])
			return fmt.Sprintf("\n- %s", text)
		}
		return match
	})

	// Convert paragraphs and divs
	tagRe := regexp.MustCompile(`(?i)</(p|div|section|article)>`)
	content = tagRe.ReplaceAllString(content, "\n\n")

	// Convert line breaks
	brRe := regexp.MustCompile(`(?i)<(br|hr)\s*/?>`)
	content = brRe.ReplaceAllString(content, "\n")

	return stripTags(content)
}
