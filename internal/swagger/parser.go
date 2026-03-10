package swagger

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getkin/kin-openapi/openapi3"
)

// Parser handles parsing of Swagger/OpenAPI documents
type Parser struct {
	httpClient *http.Client
}

// NewParser creates a new swagger parser
func NewParser() *Parser {
	return &Parser{
		httpClient: &http.Client{
			Timeout: 0, // No timeout by default
		},
	}
}

// ParseFromURL fetches and parses an OpenAPI document from a URL
func (p *Parser) ParseFromURL(docURL string) (*openapi3.T, string, error) {
	// Fetch the document
	resp, err := p.httpClient.Get(docURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch swagger doc: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch swagger doc: HTTP %d", resp.StatusCode)
	}

	// Read the body
	docData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read swagger doc: %w", err)
	}

	// Load the OpenAPI document
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(docData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse swagger doc: %w", err)
	}

	// Infer base URL from document URL if not provided
	baseURL := ""
	if doc.Servers != nil && len(doc.Servers) > 0 {
		baseURL = doc.Servers[0].URL
	} else {
		parsedURL, err := url.Parse(docURL)
		if err == nil {
			// Remove the path from the URL
			baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		}
	}

	// Validate the document
	if err := doc.Validate(loader.Context); err != nil {
		log.Warn("OpenAPI document has validation errors", "url", docURL, "error", err, "baseURL", baseURL, "doc", doc, "loader", loader)
	}

	return doc, baseURL, nil
}

// ExtractEndpoints extracts all endpoints from the OpenAPI document
func (p *Parser) ExtractEndpoints(doc *openapi3.T, cfg *Config) []*EndpointInfo {
	endpoints := []*EndpointInfo{}

	if doc.Paths == nil {
		return endpoints
	}

	// paths.Map() returns a map[string]*PathItem
	for path, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Check each HTTP method
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"PATCH":  pathItem.Patch,
			"DELETE": pathItem.Delete,
		}

		for method, operation := range operations {
			if operation == nil {
				continue
			}

			// Check tags filter
			if len(cfg.IncludeTags) > 0 {
				hasTag := false
				for _, tag := range operation.Tags {
					for _, includeTag := range cfg.IncludeTags {
						if strings.EqualFold(tag, includeTag) {
							hasTag = true
							break
						}
					}
					if hasTag {
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			// Check exclude tags
			exclude := false
			for _, tag := range operation.Tags {
				for _, excludeTag := range cfg.ExcludeTags {
					if strings.EqualFold(tag, excludeTag) {
						exclude = true
						break
					}
				}
				if exclude {
					break
				}
			}
			if exclude {
				continue
			}

			endpoints = append(endpoints, &EndpointInfo{
				Path:        path,
				Method:      method,
				Operation:   operation,
				Summary:     operation.Summary,
				Description: operation.Description,
				Tags:        operation.Tags,
			})
		}
	}

	return endpoints
}

// EndpointInfo holds information about a single endpoint
type EndpointInfo struct {
	Path        string
	Method      string
	Operation   *openapi3.Operation
	Summary     string
	Description string
	Tags        []string
}

// pathToID converts a path like "/api/users/{id}" to "api_users_id"
func pathToID(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Replace path parameters {id} with just "id"
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")

	// Replace remaining slashes with underscores
	path = strings.ReplaceAll(path, "/", "_")

	// Replace multiple consecutive underscores
	path = strings.ReplaceAll(path, "__", "_")

	return path
}
