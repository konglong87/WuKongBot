package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getkin/kin-openapi/openapi3"
)

// Generator converts OpenAPI endpoints to Tool implementations
type Generator struct {
	source        *Source
	client        *APIClient
	defaultLimit  int
	defaultOffset int
}

// NewGenerator creates a new tool generator
func NewGenerator(source *Source, client *APIClient, defaultLimit, defaultOffset int) *Generator {
	return &Generator{
		source:        source,
		client:        client,
		defaultLimit:  defaultLimit,
		defaultOffset: defaultOffset,
	}
}

// GenerateTools creates Tool implementations from endpoints
func (g *Generator) GenerateTools(endpoints []*EndpointInfo) []APITool {
	tools := []APITool{}

	for _, endpoint := range endpoints {
		tool := APITool{
			source:        g.source,
			client:        g.client,
			endpoint:      endpoint,
			defaultLimit:  g.defaultLimit,
			defaultOffset: g.defaultOffset,
		}
		tools = append(tools, tool)
	}

	return tools
}

// APITool implements the Tool interface for API endpoints
type APITool struct {
	source        *Source
	client        *APIClient
	endpoint      *EndpointInfo
	defaultLimit  int
	defaultOffset int
}

// Tag returns the first tag of the endpoint
func (t *APITool) Tag() string {
	if t.endpoint.Tags != nil && len(t.endpoint.Tags) > 0 {
		return t.endpoint.Tags[0]
	}
	return "Other"
}

// Name returns the tool name
func (t *APITool) Name() string {
	return fmt.Sprintf("api_%s_%s", strings.ToLower(t.endpoint.Method), pathToID(t.endpoint.Path))
}

// Description returns a human-readable description
func (t *APITool) Description() string {
	desc := t.endpoint.Summary
	if desc == "" {
		desc = fmt.Sprintf("%s %s", t.endpoint.Method, t.endpoint.Path)
	}

	// Add information about parameters
	if len(t.endpoint.Operation.Parameters) > 0 {
		params := []string{}
		for _, paramRef := range t.endpoint.Operation.Parameters {
			if paramRef != nil && paramRef.Value != nil && paramRef.Value.Required {
				params = append(params, paramRef.Value.Name)
			}
		}
		if len(params) > 0 {
			desc += fmt.Sprintf("\nRequired parameters: %s", strings.Join(params, ", "))
		}
	}

	// Add request body info
	if t.endpoint.Operation.RequestBody != nil && t.endpoint.Operation.RequestBody.Value != nil {
		desc += "\nThis endpoint requires a request body."
	}

	return desc
}

// Parameters returns the JSON schema for tool parameters
func (t *APITool) Parameters() json.RawMessage {
	properties := map[string]interface{}{}
	required := []string{}

	// Add path parameters
	for _, paramRef := range t.endpoint.Operation.Parameters {
		if paramRef == nil || paramRef.Value == nil || paramRef.Value.In != "path" {
			continue
		}
		param := paramRef.Value

		schema := t.schemaToJSONSchema(param.Schema)
		if param.Description != "" {
			schema["description"] = param.Description
		}

		paramName := "_path_" + param.Name
		properties[paramName] = schema
		if param.Required {
			required = append(required, paramName)
		}
	}

	// Add query parameters
	for _, paramRef := range t.endpoint.Operation.Parameters {
		if paramRef == nil || paramRef.Value == nil || paramRef.Value.In != "query" {
			continue
		}
		param := paramRef.Value

		schema := t.schemaToJSONSchema(param.Schema)
		if param.Description != "" {
			schema["description"] = param.Description
		}

		// Check if this is a pagination parameter and set default
		if isPaginationParam(param.Name) && schema["type"] == "integer" {
			if strings.Contains(strings.ToLower(param.Name), "size") ||
				strings.Contains(strings.ToLower(param.Name), "limit") {
				schema["default"] = t.defaultLimit
			}
			if strings.Contains(strings.ToLower(param.Name), "offset") {
				schema["default"] = t.defaultOffset
			}
		}

		paramName := "_query_" + param.Name
		properties[paramName] = schema
		if param.Required {
			required = append(required, paramName)
		}
	}

	// Add request body
	if t.endpoint.Operation.RequestBody != nil && t.endpoint.Operation.RequestBody.Value != nil {
		bodySchema := t.extractBodySchema(t.endpoint.Operation.RequestBody)

		// If body is an object with properties, expand its properties directly
		if bodyType, ok := bodySchema["type"].(string); ok && bodyType == "object" {
			if bodyProps, ok := bodySchema["properties"].(map[string]interface{}); ok && len(bodyProps) > 0 {
				// Merge body properties directly into the tool parameters
				for name, prop := range bodyProps {
					properties[name] = prop
				}
				if bodyReq, ok := bodySchema["required"].([]string); ok {
					required = append(required, bodyReq...)
				}
			}
		} else {
			// Otherwise, keep _body for non-object bodies
			properties["_body"] = bodySchema
			if t.endpoint.Operation.RequestBody.Value.Required {
				required = append(required, "_body")
			}
		}
	}

	paramSchema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		paramSchema["required"] = required
	}

	result, err := json.Marshal(paramSchema)
	if err != nil {
		log.Error("Failed to marshal tool parameters", "error", err)
		return json.RawMessage(`{"type": "object", "properties": {}}`)
	}

	return result
}

// Execute executes the tool with the given arguments
func (t *APITool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathParams := map[string]interface{}{}
	queryParams := map[string]interface{}{}
	bodyParams := map[string]interface{}{}

	for key, value := range args {
		if strings.HasPrefix(key, "_path_") {
			paramName := strings.TrimPrefix(key, "_path_")
			pathParams[paramName] = value
		} else if strings.HasPrefix(key, "_query_") {
			paramName := strings.TrimPrefix(key, "_query_")
			queryParams[paramName] = value
		} else if key == "_body" {
			if bodyMap, ok := value.(map[string]interface{}); ok {
				bodyParams = bodyMap
			}
		} else if key == "_body_raw" {
			// Skip _body_raw for now, handle after loop
		} else if key == "raw" {
			// Skip raw parameter used by LLM
		} else if !strings.HasPrefix(key, "_") {
			// Body properties (expanded from object body)
			bodyParams[key] = value
		}
	}

	// Check for raw JSON in _body_raw
	if bodyRaw, ok := args["_body_raw"].(string); ok && bodyRaw != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(bodyRaw), &parsed); err == nil {
			bodyParams = parsed
		}
	}

	result, err := t.client.ExecuteRequest(ctx, t.endpoint.Method, t.endpoint.Path, pathParams, queryParams, bodyParams)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}

	return result, nil
}

// ConcurrentSafe returns true - API calls are stateless and safe to run concurrently
func (t *APITool) ConcurrentSafe() bool {
	return true
}

// schemaToJSONSchema converts an OpenAPI schema to JSON Schema
func (t *APITool) schemaToJSONSchema(schemaRef *openapi3.SchemaRef) map[string]interface{} {
	if schemaRef == nil || schemaRef.Value == nil {
		return map[string]interface{}{"type": "string"}
	}

	schema := schemaRef.Value
	result := make(map[string]interface{})

	// Get the first type if available
	if schema.Type != nil && len(*schema.Type) > 0 {
		result["type"] = string((*schema.Type)[0])
	}

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if schema.Default != nil {
		result["default"] = schema.Default
	}

	// Handle enum values if present
	if schema.Enum != nil && len(schema.Enum) > 0 {
		result["enum"] = append([]interface{}{}, schema.Enum...)
	}

	// Handle nested schemas (objects)
	isObject := false
	if schema.Type != nil && len(*schema.Type) > 0 && (*schema.Type)[0] == "object" {
		isObject = true
	}
	if isObject && schema.Properties != nil {
		props := make(map[string]interface{})
		for name, propRef := range schema.Properties {
			if propRef != nil {
				props[name] = t.schemaToJSONSchema(propRef)
			}
		}
		result["properties"] = props
		if schema.Required != nil {
			result["required"] = append([]string{}, schema.Required...)
		}
	}

	// Handle arrays
	isArray := false
	if schema.Type != nil && len(*schema.Type) > 0 && (*schema.Type)[0] == "array" {
		isArray = true
	}
	if isArray && schema.Items != nil && schema.Items.Value != nil {
		result["items"] = t.schemaToJSONSchema(schema.Items)
	}

	// If no type specified, default to string
	if _, ok := result["type"]; !ok {
		result["type"] = "string"
	}

	return result
}

// extractBodySchema extracts the request body schema
func (t *APITool) extractBodySchema(requestBody *openapi3.RequestBodyRef) map[string]interface{} {
	if requestBody == nil || requestBody.Value == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}

	content := requestBody.Value.Content
	if content == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}

	// Try application/json first
	jsonContent, ok := content["application/json"]
	if ok && jsonContent != nil && jsonContent.Schema != nil {
		return t.schemaToJSONSchema(jsonContent.Schema)
	}

	// Fallback to any content type
	for contentType, mediaType := range content {
		if mediaType != nil && mediaType.Schema != nil {
			log.Debug("Using content type fallback", "type", contentType)
			return t.schemaToJSONSchema(mediaType.Schema)
		}
	}

	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

// isPaginationParam checks if a parameter name suggests pagination
func isPaginationParam(name string) bool {
	lower := strings.ToLower(name)
	paginationKeywords := []string{"page", "size", "limit", "offset", "per_page", "pagesize"}
	for _, keyword := range paginationKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// cleanDescription removes special characters and formatting from descriptions
func cleanDescription(desc string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	desc = re.ReplaceAllString(desc, "")
	// Remove excessive whitespace
	desc = strings.Join(strings.Fields(desc), " ")
	return desc
}
