package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/getkin/kin-openapi/openapi3"
)

// SwaggerService interface for API operations
type SwaggerService interface {
	// CallAPI calls an API endpoint and returns the response
	CallAPI(ctx context.Context, method, path string, params map[string]interface{}) (string, error)

	// ListAPIs returns all available API endpoints
	ListAPIs() []APIInfo

	// GetAPIDescription returns description of an API endpoint
	GetAPIDescription(method, path string) string

	// GetAPIParams returns parameter information for an API endpoint
	GetAPIParams(method, path string) (pathParams, queryParams, requestBody string)
}

// APIInfo represents an API endpoint information
type APIInfo struct {
	Method      string
	Path        string
	Summary     string
	Tag         string
	Operation   interface{} // OpenAPI operation details
	RequestBody string      // Request body schema (JSON format)
	QueryParam  string      // Query parameters (JSON format)
	PathParam   string      // Path parameters (JSON format)
}

// swaggerServiceImpl implements SwaggerService
type swaggerServiceImpl struct {
	registry *Registry
	parser   *Parser
}

// NewSwaggerService creates a new swagger service
func NewSwaggerService(registry *Registry, parser *Parser) SwaggerService {
	return &swaggerServiceImpl{
		registry: registry,
		parser:   parser,
	}
}

// CallAPI calls an API endpoint using O(1) map lookup
func (s *swaggerServiceImpl) CallAPI(ctx context.Context, method, path string, params map[string]interface{}) (string, error) {
	// O(1) lookup using map (GetToolByMethodPath handles case normalization)
	tool, exists := s.registry.GetToolByMethodPath(method, path)
	if !exists {
		// API not found - log error for debugging
		normalizedMethod := strings.ToUpper(method)
		log.Error("API endpoint not found", "method", method, "normalized_method", normalizedMethod, "path", path,
			"total_apis_available", len(s.registry.allToolsMap))

		// Show some available APIs for debugging
		availableCount := 0
		for key := range s.registry.allToolsMap {
			if availableCount < 10 {
				log.Debug("Available API", "method:path", key)
				availableCount++
			}
		}

		return "", fmt.Errorf("API endpoint not found: %s %s", normalizedMethod, path)
	}

	// Get client from the tool (APITool stores its own client)
	client := tool.client

	// Parse params
	pathParams := make(map[string]interface{})
	queryParams := make(map[string]interface{})
	bodyParams := make(map[string]interface{})

	for key, value := range params {
		if key == "_path" {
			if m, ok := value.(map[string]interface{}); ok {
				pathParams = m
			}
		} else if key == "_query" {
			if m, ok := value.(map[string]interface{}); ok {
				queryParams = m
			}
		} else if key == "_body" {
			if m, ok := value.(map[string]interface{}); ok {
				bodyParams = m
			}
		}
	}

	// Use the method from the endpoint (standardized) to ensure consistency
	return client.ExecuteRequest(ctx, tool.endpoint.Method, tool.endpoint.Path, pathParams, queryParams, bodyParams)
}

// ListAPIs returns all available API endpoints
func (s *swaggerServiceImpl) ListAPIs() []APIInfo {
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	var apis []APIInfo
	for _, state := range s.registry.sources {
		for _, tool := range state.tools {
			tag := "Other"
			if tool.endpoint.Tags != nil && len(tool.endpoint.Tags) > 0 {
				tag = tool.endpoint.Tags[0]
			}

			// Extract parameter information directly from the operation
			paramsInfo := s.extractParamsFromOperation(tool.endpoint)

			apis = append(apis, APIInfo{
				Method:      tool.endpoint.Method,
				Path:        tool.endpoint.Path,
				Summary:     tool.endpoint.Summary,
				Tag:         tag,
				Operation:   tool.endpoint.Operation,
				RequestBody: paramsInfo.requestBody,
				QueryParam:  paramsInfo.queryParams,
				PathParam:   paramsInfo.pathParams,
			})
		}
	}
	return apis
}

// GetAPIDescription returns description of an API endpoint
func (s *swaggerServiceImpl) GetAPIDescription(method, path string) string {
	normalizeMethod := strings.ToUpper(method)
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	for _, state := range s.registry.sources {
		for _, tool := range state.tools {
			if tool.endpoint.Method == normalizeMethod && tool.endpoint.Path == path {
				desc := tool.endpoint.Summary
				if desc == "" {
					desc = tool.endpoint.Method + " " + tool.endpoint.Path
				}
				return desc
			}
		}
	}
	return normalizeMethod + " " + path
}

// GetAPIParams returns parameter information for an API endpoint
func (s *swaggerServiceImpl) GetAPIParams(method, path string) (pathParams, queryParams, requestBody string) {
	normalizeMethod := strings.ToUpper(method)
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()

	for _, state := range s.registry.sources {
		for _, tool := range state.tools {
			if tool.endpoint.Method == normalizeMethod && tool.endpoint.Path == path {
				paramsInfo := s.extractParamsFromOperation(tool.endpoint)
				return paramsInfo.pathParams, paramsInfo.queryParams, paramsInfo.requestBody
			}
		}
	}
	return "", "", ""
}

// parametersInfo holds parameter information
type parametersInfo struct {
	pathParams  string
	queryParams string
	requestBody string
}

// extractParamsFromOperation extracts parameters directly from OpenAPI operation
func (s *swaggerServiceImpl) extractParamsFromOperation(endpoint *EndpointInfo) parametersInfo {
	info := parametersInfo{}
	if endpoint.Operation == nil {
		return info
	}

	pathParamFields := map[string]interface{}{}
	queryParamFields := map[string]interface{}{}
	bodyParamFields := map[string]interface{}{}
	hasBodyParam := false

	// Extract operation parameters
	if len(endpoint.Operation.Parameters) > 0 {
		for _, paramRef := range endpoint.Operation.Parameters {
			if paramRef == nil || paramRef.Value == nil {
				continue
			}
			param := paramRef.Value

			field := map[string]interface{}{
				"type":        "string",
				"description": param.Description,
			}

			// Set type from schema if available
			if param.Schema != nil && param.Schema.Value != nil {
				schema := param.Schema.Value
				if schema.Type != nil {
					field["type"] = *schema.Type
				}
				if required := param.Required; required {
					field["required"] = true
				}
			}

			switch param.In {
			case "path":
				pathParamFields[param.Name] = field
			case "query":
				queryParamFields[param.Name] = field
			case "body":
				hasBodyParam = true
				// Handle body parameters - try to get properties from schema
				if param.Schema != nil {
					schemaMap := s.schemaToSimpleMap(param.Schema)
					if len(schemaMap) > 0 {
						// Merge schema properties into bodyParamFields
						for k, v := range schemaMap {
							bodyParamFields[k] = v
						}
						log.Debug("Extracted body param schema", "param", param.Name, "fields", len(schemaMap), "path", endpoint.Path)
					} else {
						// Schema has $ref or is empty - add a note
						hasRef := param.Schema.Ref != ""
						log.Debug("Body parameter schema is empty", "path", endpoint.Path, "has_ref", hasRef, "ref", param.Schema.Ref, "name", param.Name)
						bodyParamFields["__note__"] = map[string]interface{}{
							"type":        "string",
							"description": "Request body is required (parameter: " + param.Name + "). The API may reference a schema definition.",
						}
					}
				}
			}
		}
	}

	// If we have a body param with empty schema, still show that body is required
	if hasBodyParam && len(bodyParamFields) == 0 {
		bodyParamFields["__body__"] = map[string]interface{}{
			"type":        "object",
			"description": "Request body is required. Check API documentation for exact format.",
		}
	}

	// Also extract from standard requestBody (if present)
	if endpoint.Operation.RequestBody != nil && endpoint.Operation.RequestBody.Value != nil {
		reqBody := endpoint.Operation.RequestBody.Value
		if reqBody.Content != nil {
			mediaType := reqBody.Content["application/json"]
			if mediaType != nil && mediaType.Schema != nil {
				schema := s.schemaToSimpleMap(mediaType.Schema)
				for k, v := range schema {
					bodyParamFields[k] = v
				}

				// Apply required fields
				if reqBody.Required && mediaType.Schema.Value != nil {
					for _, propName := range mediaType.Schema.Value.Required {
						if propField, ok := bodyParamFields[propName]; ok {
							if propMap, ok := propField.(map[string]interface{}); ok {
								propMap["required"] = true
							}
						}
					}
				}
			}
		}
	}

	log.Debug("Extracted params from operation", "path", endpoint.Path, "body_count", len(bodyParamFields), "has_body_param", hasBodyParam)

	// Serialize to JSON string for display
	if len(pathParamFields) > 0 {
		if b, err := json.MarshalIndent(pathParamFields, "  ", "  "); err == nil {
			info.pathParams = string(b)
		}
	}
	if len(queryParamFields) > 0 {
		if b, err := json.MarshalIndent(queryParamFields, "  ", "  "); err == nil {
			info.queryParams = string(b)
		}
	}
	if len(bodyParamFields) > 0 {
		if b, err := json.MarshalIndent(bodyParamFields, "  ", "  "); err == nil {
			info.requestBody = string(b)
		}
	}

	return info
}

// schemaToSimpleMap converts OpenAPI schema to simple map
func (s *swaggerServiceImpl) schemaToSimpleMap(schemaRef *openapi3.SchemaRef) map[string]interface{} {
	result := make(map[string]interface{})
	if schemaRef == nil || schemaRef.Value == nil {
		log.Debug("schemaToSimpleMap: nil schema")
		return result
	}
	schema := schemaRef.Value

	log.Debug("schemaToSimpleMap: parsing schema", "has_properties", schema.Properties != nil, "property_count", len(schema.Properties))

	if schema.Properties != nil {
		for name, propRef := range schema.Properties {
			if propRef == nil || propRef.Value == nil {
				continue
			}
			prop := propRef.Value

			field := map[string]interface{}{
				"type":        "string",
				"description": prop.Description,
			}

			if prop.Type != nil {
				field["type"] = *prop.Type
			}
			if prop.Default != nil {
				field["default"] = prop.Default
			}

			result[name] = field
		}
	}

	return result
}

// APIError represents an API error
type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

var (
	// ErrAPINotFound is returned when an API endpoint is not found
	ErrAPINotFound = &APIError{Message: "API endpoint not found"}
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
