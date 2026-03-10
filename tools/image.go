package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// ImageGenerator is the interface for image generation models
type ImageGenerator interface {
	BuildRequest(prompt string, params map[string]interface{}) ([]byte, error)
	ParseResponse(body []byte) ([]string, string, error)
	GetPollURL(taskID string) string
}

// QwenImageGenerator implements ImageGenerator for Qwen image models
// API: https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation
type QwenImageGenerator struct {
	model string
}

// NewQwenImageGenerator creates a new Qwen image generator
func NewQwenImageGenerator(model string) *QwenImageGenerator {
	if model == "" {
		model = "qwen-image-max"
	}
	return &QwenImageGenerator{model: model}
}

// BuildRequest builds the request body for Qwen Image API
func (g *QwenImageGenerator) BuildRequest(prompt string, params map[string]interface{}) ([]byte, error) {
	requestBody := map[string]interface{}{
		"model": g.model,
		"input": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"text": prompt,
						},
					},
				},
			},
		},
		"parameters": make(map[string]interface{}),
	}

	// Register default values
	paramsMap := requestBody["parameters"].(map[string]interface{})

	// size (图片尺寸)
	if size, ok := params["size"].(string); ok && size != "" {
		paramsMap["size"] = size
	} else {
		paramsMap["size"] = "1664*928" // qwen-image-max default
	}

	// negative_prompt (负面提示)
	if negative, ok := params["negative_prompt"].(string); ok && negative != "" {
		paramsMap["negative_prompt"] = negative
	}

	// prompt_extend (提示词扩展)
	if promptExtend, ok := params["prompt_extend"].(bool); ok {
		paramsMap["prompt_extend"] = promptExtend
	} else {
		paramsMap["prompt_extend"] = true // default true
	}

	// watermark (水印)
	if watermark, ok := params["watermark"].(bool); ok {
		paramsMap["watermark"] = watermark
	} else {
		paramsMap["watermark"] = false // default false
	}

	return json.Marshal(requestBody)
}

// ParseResponse parses the Qwen Image API response
func (g *QwenImageGenerator) ParseResponse(body []byte) ([]string, string, error) {
	var result struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("parse error: %v", err)
	}

	// Check API error
	if result.Code != "" && result.Code != "Success" {
		return nil, "", fmt.Errorf("API error: %s (code: %s)", result.Message, result.Code)
	}

	// Extract image URLs
	var urls []string
	for _, choice := range result.Output.Choices {
		for _, content := range choice.Message.Content {
			if content.Image != "" {
				urls = append(urls, content.Image)
			}
		}
	}

	return urls, "", nil
}

// GetPollURL returns the URL to poll for the async task
func (g *QwenImageGenerator) GetPollURL(taskID string) string {
	// Qwen Image API doesn't use async polling, returns result directly
	return ""
}

// WanXGenerator implements ImageGenerator for WanX models (legacy)
// API: https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/generation
type WanXGenerator struct {
	model string
}

// NewWanXGenerator creates a new WanX generator
func NewWanXGenerator(model string) *WanXGenerator {
	if model == "" {
		model = "wanx-v1"
	}
	return &WanXGenerator{model: model}
}

// BuildRequest builds the request body for WanX API
func (g *WanXGenerator) BuildRequest(prompt string, params map[string]interface{}) ([]byte, error) {
	requestBody := map[string]interface{}{
		"model": g.model,
		"input": map[string]interface{}{
			"prompt": prompt,
		},
		"parameters": make(map[string]interface{}),
	}

	// Set parameters
	paramsMap := requestBody["parameters"].(map[string]interface{})

	if size, ok := params["size"].(string); ok && size != "" {
		paramsMap["size"] = size
	}
	if n, ok := params["n"].(int); ok && n > 0 {
		paramsMap["n"] = n
	}

	return json.Marshal(requestBody)
}

// ParseResponse parses the WanX API response
func (g *WanXGenerator) ParseResponse(body []byte) ([]string, string, error) {
	var result struct {
		Output struct {
			TaskID  string `json:"task_id"`
			Results []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", fmt.Errorf("parse error: %v", err)
	}

	// Check API error
	if result.Code != "" {
		return nil, "", fmt.Errorf("API error: %s (code: %s)", result.Message, result.Code)
	}

	// Check if async task was created
	if result.Output.TaskID != "" {
		return nil, result.Output.TaskID, nil
	}

	// Sync mode - extract URLs
	var urls []string
	for _, r := range result.Output.Results {
		if r.URL != "" {
			urls = append(urls, r.URL)
		}
	}
	return urls, "", nil
}

// GetPollURL returns the URL to poll for the async task
func (g *WanXGenerator) GetPollURL(taskID string) string {
	return fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)
}

// ImageGenerationTool generates images using text-to-image models
type ImageGenerationTool struct {
	apiKey  string
	apiBase string
	model   string
	models  map[string]string // Model-specific API bases
	client  *http.Client
}

// NewImageGenerationTool creates a new image generation tool
func NewImageGenerationTool(apiKey, apiBase, model string, models map[string]string) *ImageGenerationTool {
	if apiKey == "" {
		log.Warn("ImageGenerationTool created with empty API key - tool will not work")
	}

	// Determine actual API base
	actualAPIBase := apiBase
	if actualAPIBase == "" && model != "" && models != nil {
		if url, ok := models[model]; ok {
			actualAPIBase = url
			log.Info("ImageGenerationTool using model-specific API base", "model", model, "url", url)
		}
	}

	// Default endpoints if still not set
	if actualAPIBase == "" {
		actualAPIBase = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
		if model == "" {
			model = "qwen-image-max"
		}
	}

	if models == nil {
		models = make(map[string]string)
	}

	return &ImageGenerationTool{
		apiKey:  apiKey,
		apiBase: actualAPIBase,
		model:   model,
		models:  models,
		client: &http.Client{
			Timeout: 60 * time.Second, // Image generation may take longer
		},
	}
}

// Name returns the tool name
func (t *ImageGenerationTool) Name() string {
	return "generate_image"
}

// Description returns the tool description
func (t *ImageGenerationTool) Description() string {
	return "Generate an image from text description (文生图). Supported models: qwen-image-max (recommended), qwen-image-v1, wanx-v1, wanx-v2, wanx-sketch. Image URLs expire after 24 hours."
}

// Parameters returns the JSON schema for parameters
func (t *ImageGenerationTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "The text description of the image to generate (文字描述)"
			},
			"size": {
				"type": "string",
				"description": "Image size (图片尺寸). For qwen-image-max: '1024*1024', '1280*720', '1664*928', '720*1280'.",
				"enum": ["1024*1024", "1280*720", "1664*928", "720*1280", "768*1344", "864*1152", "480*960", "512*512"]
			},
			"negative_prompt": {
				"type": "string",
				"description": "Negative prompt describing what to avoid (负面提示词，可选)"
			},
			"watermark": {
				"type": "boolean",
				"description": "Add watermark to image (是否添加水印，默认false)"
			}
		},
		"required": ["prompt"]
	}`)
}

// ConcurrentSafe returns true - image generation is stateless
func (t *ImageGenerationTool) ConcurrentSafe() bool {
	return true
}

// Execute generates an image from text prompt
func (t *ImageGenerationTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.apiKey == "" {
		return "Error: ImageGenerationTool is not configured with an API key", nil
	}

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return "Error: prompt is required", nil
	}

	// Determine which generator to use based on API base or model
	var generator ImageGenerator
	var requestBody []byte

	// Check if using Qwen Image API (multimodal-generation)
	if strings.Contains(t.apiBase, "multimodal-generation") {
		generator = NewQwenImageGenerator(t.model)
	} else {
		// Legacy WanX API (text2image)
		generator = NewWanXGenerator(t.model)
	}

	// Build parameters map
	params := make(map[string]interface{})

	if size, ok := args["size"].(string); ok && size != "" {
		params["size"] = size
	}

	if negative, ok := args["negative_prompt"].(string); ok && negative != "" {
		params["negative_prompt"] = negative
	}

	if watermark, ok := args["watermark"].(bool); ok {
		params["watermark"] = watermark
	}

	// Build request body
	var err error
	requestBody, err = generator.BuildRequest(prompt, params)
	if err != nil {
		return fmt.Sprintf("Error building request: %v", err), nil
	}

	log.Debug("Image generation request", "model", t.model, "url", t.apiBase, "body", string(requestBody))

	req, err := http.NewRequestWithContext(ctx, "POST", t.apiBase, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Sprintf("Error creating request: %v", err), nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error making request: %v", err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Error reading response: %v", err), nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Error: API returned status %d: %s", resp.StatusCode, string(body)), nil
	}

	// Parse response
	urls, taskID, err := generator.ParseResponse(body)
	if err != nil {
		return fmt.Sprintf("Error parsing response: %v (body: %s)", err, string(body)), nil
	}

	// If task ID returned, poll for async result (WanX legacy API)
	if taskID != "" {
		return t.pollForResult(ctx, taskID)
	}

	// Sync mode - return URLs directly with marking for channel parsing
	if len(urls) > 0 {
		// Add [IMAGE]...[/IMAGE] tags for channel to parse and display images
		imageBlocks := ""
		for _, url := range urls {
			imageBlocks += fmt.Sprintf("[IMAGE]%s[/IMAGE]\n", url)
		}
		return fmt.Sprintf("%s提示: 图片URL仅保留24小时，请及时保存！", imageBlocks), nil
	}

	return fmt.Sprintf("No image generated. Response: %s", string(body)), nil
}

// pollForResult polls the async task for the result (WanX legacy API)
func (t *ImageGenerationTool) pollForResult(ctx context.Context, taskID string) (string, error) {
	pollURL := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	maxAttempts := 30
	attempt := 0

	for attempt < maxAttempts {
		attempt++

		req, err := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		if err != nil {
			return fmt.Sprintf("Error creating poll request: %v", err), nil
		}

		req.Header.Set("Authorization", "Bearer "+t.apiKey)

		resp, err := t.client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error polling result: %v", err), nil
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Sprintf("Error reading poll response: %v", err), nil
		}

		if resp.StatusCode != http.StatusOK {
			// Try again on transient errors
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				time.Sleep(2 * time.Second)
				continue
			}
			return fmt.Sprintf("Error: API returned status %d: %s", resp.StatusCode, string(body)), nil
		}

		var result struct {
			Task struct {
				Status string `json:"task_status"`
			} `json:"task"`
			Output struct {
				Results []struct {
					URL string `json:"url"`
				} `json:"results"`
			} `json:"output,omitempty"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Sprintf("Error parsing poll response: %v", err), nil
		}

		log.Debug("Poll result", "attempt", attempt, "status", result.Task.Status)

		switch result.Task.Status {
		case "SUCCEEDED":
			if len(result.Output.Results) > 0 {
				var urls []string
				for _, r := range result.Output.Results {
					if r.URL != "" {
						urls = append(urls, r.URL)
					}
				}
				// Add [IMAGE]...[/IMAGE] tags for channel to parse and display images
				imageBlocks := ""
				for _, url := range urls {
					imageBlocks += fmt.Sprintf("[IMAGE]%s[/IMAGE]\n", url)
				}
				return fmt.Sprintf("%s提示: 图片URL仅保留24小时，请及时保存！", imageBlocks), nil
			}
			return "Task succeeded but no images returned", nil
		case "FAILED":
			return fmt.Sprintf("Image generation failed: %s", string(body)), nil
		case "PENDING", "RUNNING":
			time.Sleep(2 * time.Second)
			continue
		}
	}

	return "Timeout waiting for image generation result", nil
}

// ImageAnalysisTool analyzes images using Qwen-VL (Qwen's vision-language model)
type ImageAnalysisTool struct {
	apiKey  string
	apiBase string
	model   string
	client  *http.Client
}

// NewImageAnalysisTool creates a new image analysis tool
func NewImageAnalysisTool(apiKey, apiBase, model string) *ImageAnalysisTool {
	if apiKey == "" {
		log.Warn("ImageAnalysisTool created with empty API key - tool will not work")
	}
	if apiBase == "" {
		apiBase = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	}
	if model == "" {
		model = "qwen-vl-max"
	}

	return &ImageAnalysisTool{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// Name returns the tool name
func (t *ImageAnalysisTool) Name() string {
	return "analyze_image"
}

// Description returns the tool description
func (t *ImageAnalysisTool) Description() string {
	return "Analyze an image using Qwen-VL (Qwen's vision-language model). Describe what's in the image, answer questions about it. (图生文)"
}

// Parameters returns the JSON schema for parameters
func (t *ImageAnalysisTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"image_url": {
				"type": "string",
				"description": "HTTP/HTTPS URL of the image to analyze (图片URL). Use ONLY for regular URLs like 'https://example.com/image.jpg'. DO NOT use for data:image/;base64, format - use image_base64 instead."
			},
			"image_base64": {
				"type": "string",
				"description": "Base64 encoded image data WITHOUT the 'data:image/jpeg;base64,' prefix. For example: '/9j/4AAQSkZJRg...' (Base64编码的图片数据，不含data:image前缀)"
			},
			"image_mime": {
				"type": "string",
				"description": "MIME type of the base64 image (e.g., 'image/jpeg', 'image/png'). Required when using image_base64."
			},
			"question": {
				"type": "string",
				"description": "Question to ask about the image (可选，若不提供则默认描述图片)"
			}
		},
		"required": []
	}`)
}

// ConcurrentSafe returns true - image analysis is stateless
func (t *ImageAnalysisTool) ConcurrentSafe() bool {
	return true
}

// Execute analyzes an image
func (t *ImageAnalysisTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.apiKey == "" {
		return "Error: ImageAnalysisTool is not configured with an API key", nil
	}
	log.Info("[analyze_image] start:  Execute Executing image analysis tool")
	// Debug: Log received args fully
	argsJSON, _ := json.Marshal(args)
	log.Info("[analyze_image] Received args", "args", string(argsJSON))

	// Check if raw field exists and is malformed
	if rawValue, ok := args["raw"]; ok {
		if rawStr, ok := rawValue.(string); ok {
			log.Info("[analyze_image] raw field found", "raw_length", len(rawStr))
		}
	}

	imageURL := ""
	imageBase64 := ""
	imageMime := "image/jpeg"

	// Get image URL or base64 data
	if url, ok := args["image_url"].(string); ok && url != "" {
		// Check if image_url contains a data URI (common mistake by LLM)
		if strings.HasPrefix(url, "data:image/") {
			// Parse data URI format: data:image/jpeg;base64,<base64_data>
			parts := strings.SplitN(url, ";", 2)
			if len(parts) == 2 {
				// Extract mime type
				if strings.HasPrefix(parts[0], "data:image/") {
					imageMime = strings.TrimPrefix(parts[0], "data:")
				}
				// Extract base64 data
				base64Parts := strings.SplitN(parts[1], ",", 2)
				if len(base64Parts) == 2 && strings.HasPrefix(base64Parts[0], "base64") {
					imageBase64 = base64Parts[1]
					log.Info("[analyze_image] Detected data URI in image_url parameter, extracting base64", "mime", imageMime, "base64_size", len(imageBase64))
				}
			}
		} else {
			imageURL = url
		}
	}
	// If not data URI, check image_base64 parameter
	if imageBase64 == "" {
		if data, ok := args["image_base64"].(string); ok && data != "" {
			imageBase64 = data
			if mime, ok := args["image_mime"].(string); ok && mime != "" {
				imageMime = mime
			}
		}
	}
	// Check if we have valid input
	if imageURL == "" && imageBase64 == "" {
		return "Error: either image_url (regular URL) or image_base64 is required", nil
	}

	// Get question (optional)
	question := "描述这张图片的详细内容。"
	if q, ok := args["question"].(string); ok && q != "" {
		question = q
	}

	// Build the message with image content (OpenAI compatible format)
	content := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": question,
		},
	}

	if imageURL != "" {
		// Use URL
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": imageURL,
			},
		})
	} else {
		// Use base64
		dataURL := fmt.Sprintf("data:%s;base64,%s", imageMime, imageBase64)
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": dataURL,
			},
		})
	}

	// Build request body (OpenAI compatible format)
	requestBody := map[string]interface{}{
		"model": t.model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	log.Info("[analyze_image] Sending request to API", "url", t.apiBase, "question", question, "requestBody", requestBody, "image_url", imageURL, "image_base64", imageBase64, "image_mime", imageMime)
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Sprintf("Error marshaling request: %v", err), nil
	}

	log.Info("[analyze_image] Request body", "body", string(jsonData))

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiBase, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Sprintf("Error creating request: %v", err), nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error making request: %v", err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Error reading response: %v", err), nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("【ImageAnalysisTool】Error: API returned status %d: %s", resp.StatusCode, string(body)), nil
	}

	// Parse response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Sprintf("ImageAnalysisTool Error parsing response: %v\nRaw response: %s", err, string(body)), nil
	}

	if len(result.Choices) == 0 {
		return "ImageAnalysisTool No response from the model", nil
	}

	return result.Choices[0].Message.Content, nil
}
