package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// SwaggerTool provides access to API endpoints via Swagger/OpenAPI specifications
type SwaggerTool struct {
	swaggerService SwaggerService
	mu             sync.RWMutex
}

// NewSwaggerTool creates a new swagger tool
func NewSwaggerTool(swaggerService SwaggerService) *SwaggerTool {
	return &SwaggerTool{
		swaggerService: swaggerService,
	}
}

// Name returns the tool name
func (t *SwaggerTool) Name() string {
	return "api"
}

// Description returns the tool description
func (t *SwaggerTool) Description() string {
	return "Execute HTTP API calls to EXTERNAL web services via Swagger/OpenAPI defined endpoints. " +
		"Use this ONLY for making HTTP requests to external APIs defined in Swagger specifications. " +
		"DO NOT use for: local database queries, local file operations, or any local system operations. " +
		"For local SQLite databases, use the 'exec' tool with sqlite3 commands directly."
}

// Parameters returns the JSON schema for parameters
func (t *SwaggerTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "call", "search"],
				"description": "Action: list (show all available APIs), call (execute a specific API endpoint), search (search for APIs by keyword or tag)"
			},
			"method": {
				"type": "string",
				"description": "HTTP method for the API call (required for call action): GET, POST, PUT, DELETE, etc."
			},
			"path": {
				"type": "string",
				"description": "API path (required for call action): e.g., '/reminder/getReminderEventList', '/user/getPatientFuzzyQuery'"
			},
			"path_params": {
				"type": "object",
				"description": "Path parameters (optional, for call action): e.g., {\"userId\": \"123\"} for /users/{userId}"
			},
			"query_params": {
				"type": "object",
				"description": "Query parameters (optional, for call action): e.g., {\"page\": 1, \"size\": 10} for ?page=1&size=10"
			},
			"body": {
				"type": "object",
				"description": "Request body parameters (optional, for call action): e.g., {\"phone\": \"13800138000\", \"captcha\": \"123456\"} for POST requests"
			},
			"keyword": {
				"type": "string",
				"description": "Search keyword (required for search action): searches API path and summary"
			},
			"tag": {
				"type": "string",
				"description": "Filter by API tag (optional, for list and search actions): e.g., 'Reminder', 'User', 'medicalReport'"
			}
		},
		"required": ["action"]
	}`)
}

// Execute performs the API action
func (t *SwaggerTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "Error: action is required (list, call, or search)", nil
	}

	switch action {
	case "list":
		return t.listAPIs(args)
	case "call":
		return t.callAPI(ctx, args)
	case "search":
		return t.searchAPIs(args)
	default:
		return "Unknown action: " + action + ". Use: list, call, or search", nil
	}
}

// listAPIs lists all available APIs
func (t *SwaggerTool) listAPIs(args map[string]interface{}) (string, error) {
	t.mu.RLock()
	swaggerService := t.swaggerService
	t.mu.RUnlock()

	if swaggerService == nil {
		return "Error: swagger service not configured", nil
	}

	apis := swaggerService.ListAPIs()
	if len(apis) == 0 {
		return "No APIs available.", nil
	}

	// Filter by tag if provided
	tagFilter := ""
	if tag, ok := args["tag"].(string); ok && tag != "" {
		tagFilter = strings.ToLower(tag)
	}

	var lines []string
	for _, api := range apis {
		if tagFilter == "" || strings.ToLower(api.Tag) == tagFilter {
			lines = append(lines, fmt.Sprintf("- %s %s - %s [tag: %s]",
				api.Method, api.Path, api.Summary, api.Tag))

			// Show parameters if they exist
			if api.RequestBody != "" {
				lines = append(lines, fmt.Sprintf("  Body parameters:\n%s", api.RequestBody))
			}
			if api.QueryParam != "" {
				lines = append(lines, fmt.Sprintf("  Query parameters:\n%s", api.QueryParam))
			}
			if api.PathParam != "" {
				lines = append(lines, fmt.Sprintf("  Path parameters:\n%s", api.PathParam))
			}
			lines = append(lines, "")
		}
	}

	if len(lines) == 0 {
		return fmt.Sprintf("No APIs found for tag: %s", tagFilter), nil
	}

	var msg string
	if tagFilter != "" {
		msg = fmt.Sprintf("Available APIs (tag: %s):\n", tagFilter)
	} else {
		msg = "Available APIs:\n"
	}
	return msg + strings.Join(lines, "\n"), nil
}

// callAPI executes an API call
func (t *SwaggerTool) callAPI(ctx context.Context, args map[string]interface{}) (string, error) {
	t.mu.RLock()
	swaggerService := t.swaggerService
	t.mu.RUnlock()

	if swaggerService == nil {
		return "Error: swagger service not configured", nil
	}

	method, ok := args["method"].(string)
	if !ok || method == "" {
		return "Error: method is required (GET, POST, PUT, DELETE, etc.)", nil
	}

	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "Error: path is required (e.g., '/reminder/getReminderEventList')", nil
	}

	// Get parameter information for this API
	_, _, requestBody := swaggerService.GetAPIParams(method, path)

	// Build params
	hasParams := false
	params := make(map[string]interface{})

	if pathParamsMap, ok := args["path_params"].(map[string]interface{}); ok && len(pathParamsMap) > 0 {
		params["_path"] = pathParamsMap
		hasParams = true
	}

	if queryParamsMap, ok := args["query_params"].(map[string]interface{}); ok && len(queryParamsMap) > 0 {
		params["_query"] = queryParamsMap
		hasParams = true
	}

	if bodyMap, ok := args["body"].(map[string]interface{}); ok && len(bodyMap) > 0 {
		params["_body"] = bodyMap
		hasParams = true
	}

	// If this is a POST/PUT without body and requestBody schema exists, show required parameters
	normalizedMethod := strings.ToUpper(method)
	if (normalizedMethod == "POST" || normalizedMethod == "PUT") && !hasParams && requestBody != "" {
		warning := fmt.Sprintf("⚠️ **Missing request body parameters for %s %s**\n\nRequired/Available body parameters:\n%s\n\nExample usage:\n"+
			"{\n  \"action\": \"call\",\n  \"method\": \"%s\",\n  \"path\": \"%s\",\n  \"body\": {\"page\": 1, \"pageSize\": 10}\n}",
			normalizedMethod, path, requestBody, normalizedMethod, path)
		return warning, nil
	}

	// Call the API
	result, err := swaggerService.CallAPI(ctx, method, path, params)
	if err != nil {
		// Return error message so LLM can understand what went wrong
		return fmt.Sprintf("Error calling API: %v", err), nil
	}

	return result, nil
}

// searchAPIs searches for APIs by keyword
func (t *SwaggerTool) searchAPIs(args map[string]interface{}) (string, error) {
	t.mu.RLock()
	swaggerService := t.swaggerService
	t.mu.RUnlock()

	if swaggerService == nil {
		return "Error: swagger service not configured", nil
	}

	apis := swaggerService.ListAPIs()
	if len(apis) == 0 {
		return "No APIs available.", nil
	}

	keyword := ""
	if kw, ok := args["keyword"].(string); ok {
		keyword = strings.ToLower(kw)
	}

	if keyword == "" {
		return "Error: keyword is required for search action", nil
	}

	// Filter by tag if provided
	tagFilter := ""
	if tag, ok := args["tag"].(string); ok && tag != "" {
		tagFilter = strings.ToLower(tag)
	}

	var lines []string
	for _, api := range apis {
		matches := false

		// Search in path
		if strings.Contains(strings.ToLower(api.Path), keyword) {
			matches = true
		}

		// Search in summary
		if strings.Contains(strings.ToLower(api.Summary), keyword) {
			matches = true
		}

		// Search in tag
		if strings.Contains(strings.ToLower(api.Tag), keyword) {
			matches = true
		}

		// Apply tag filter
		if tagFilter != "" && strings.ToLower(api.Tag) != tagFilter {
			matches = false
		}

		if matches {
			lines = append(lines, fmt.Sprintf("- %s %s - %s [tag: %s]",
				api.Method, api.Path, api.Summary, api.Tag))

			// Show parameters
			if api.RequestBody != "" {
				lines = append(lines, fmt.Sprintf("  Body parameters:\n%s", api.RequestBody))
			}
			if api.QueryParam != "" {
				lines = append(lines, fmt.Sprintf("  Query parameters:\n%s", api.QueryParam))
			}
			if api.PathParam != "" {
				lines = append(lines, fmt.Sprintf("  Path parameters:\n%s", api.PathParam))
			}
			lines = append(lines, "")
		}
	}

	if len(lines) == 0 {
		var filterMsg string
		if tagFilter != "" {
			filterMsg = fmt.Sprintf(" with tag '%s'", tagFilter)
		}
		return fmt.Sprintf("No APIs found for keyword '%s'%s", keyword, filterMsg), nil
	}

	var msg string
	if tagFilter != "" {
		msg = fmt.Sprintf("APIs matching '%s' (tag: %s):\n", keyword, tagFilter)
	} else {
		msg = fmt.Sprintf("APIs matching '%s':\n", keyword)
	}
	return msg + strings.Join(lines, "\n"), nil
}

// ConcurrentSafe returns true - API calls are stateless and safe to run concurrently
func (t *SwaggerTool) ConcurrentSafe() bool {
	return true
}
