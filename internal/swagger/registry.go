package swagger

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/konglong87/wukongbot/tools"
)

// Registry manages Swagger sources and their generated tools
type Registry struct {
	cfg         *Config
	toolReg     *tools.ToolRegistry // Kept for backward compatibility, but not used in new architecture
	parser      *Parser
	sources     map[string]*sourceState
	allToolsMap map[string]*APITool // Global map for O(1) API lookup, key: "method:path"
	mu          sync.RWMutex
}

// sourceState holds the state of a Swagger source
type sourceState struct {
	source      *Source
	client      *APIClient
	generator   *Generator
	tools       []APITool
	toolsMap    map[string]*APITool // key: "method:path" for O(1) lookup
	refreshTime time.Time
}

// NewRegistry creates a new Swagger registry
func NewRegistry(cfg *Config, toolReg *tools.ToolRegistry) *Registry {
	return &Registry{
		cfg:         cfg,
		toolReg:     toolReg,
		parser:      NewParser(),
		sources:     make(map[string]*sourceState),
		allToolsMap: make(map[string]*APITool),
	}
}

// Initialize initializes the registry with configured sources
func (r *Registry) Initialize() error {
	for i := range r.cfg.Sources {
		source := &r.cfg.Sources[i]
		if !source.Enabled {
			log.Info("Skipping disabled Swagger source", "name", source.Name)
			continue
		}

		if err := r.LoadSource(source); err != nil {
			log.Error("Failed to load Swagger source", "name", source.Name, "error", err)
			continue
		}
	}

	// Start background refresh goroutine
	go r.refreshLoop()

	return nil
}

// LoadSource loads a Swagger source and registers its tools
func (r *Registry) LoadSource(source *Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Info("Loading Swagger source", "name", source.Name, "url", source.URL)

	// Parse the Swagger document
	doc, inferredBaseURL, err := r.parser.ParseFromURL(source.URL)
	if err != nil {
		return fmt.Errorf("failed to parse swagger doc: %w", err)
	}

	// Use provided base URL or inferred one
	baseURL := source.BaseURL
	if baseURL == "" {
		baseURL = inferredBaseURL
	}

	// Create API client
	log.Debug("Creating API client", "refresk_url", source.AuthConfig.RefreshURL, "captcha_url", source.AuthConfig.CaptchaURL, "phone", source.AuthConfig.Phone)
	client := NewAPIClient(source.AuthConfig, baseURL, r.cfg.DefaultLimit, r.cfg.DefaultOffset)

	// Create generator
	generator := NewGenerator(source, client, r.cfg.DefaultLimit, r.cfg.DefaultOffset)

	// Extract endpoints
	endpoints := r.parser.ExtractEndpoints(doc, r.cfg)

	// Limit number of endpoints
	if len(endpoints) > r.cfg.MaxEndpoints {
		log.Info("Limiting number of endpoints", "total", len(endpoints), "limit", r.cfg.MaxEndpoints)
		endpoints = endpoints[:r.cfg.MaxEndpoints]
	}

	// Generate API tools (these are NOT registered to ToolRegistry,
	// just stored in registry for SwaggerService to use)
	apiTools := generator.GenerateTools(endpoints)

	// Build toolsMap for O(1) lookup within this source
	toolsMap := make(map[string]*APITool, len(apiTools))
	for i := range apiTools {
		// Use uppercase method for case-insensitive lookup
		key := strings.ToUpper(apiTools[i].endpoint.Method) + ":" + apiTools[i].endpoint.Path
		toolsMap[key] = &apiTools[i]
		// Also add to global map
		r.allToolsMap[key] = &apiTools[i]
	}

	// Store source state (tools stored here for SwaggerService lookup)
	r.sources[source.ID] = &sourceState{
		source:      source,
		client:      client,
		generator:   generator,
		tools:       apiTools,
		toolsMap:    toolsMap,
		refreshTime: time.Now(),
	}

	log.Info("Swagger source loaded", "name", source.Name, "endpoints", len(apiTools))

	return nil
}

// unregisterSourceTools removes all tools for a source
func (r *Registry) unregisterSourceTools(sourceID string) {
	if state, exists := r.sources[sourceID]; exists {
		for _, tool := range state.tools {
			// Note: ToolRegistry doesn't have an Unregister method, so we'll need to add that
			// For now, we'll log a warning
			log.Debug("Would unregister tool", "name", tool.Name())
		}
	}
}

// GetSourceStatus returns the status of all sources
func (r *Registry) GetSourceStatus() []SourceStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := []SourceStatus{}
	for _, state := range r.sources {
		status = append(status, SourceStatus{
			ID:          state.source.ID,
			Name:        state.source.Name,
			URL:         state.source.URL,
			Enabled:     state.source.Enabled,
			Endpoints:   len(state.tools),
			RefreshedAt: state.refreshTime,
		})
	}
	return status
}

// SourceStatus represents the status of a Swagger source
type SourceStatus struct {
	ID          string
	Name        string
	URL         string
	Enabled     bool
	Endpoints   int
	RefreshedAt time.Time
}

// refreshLoop periodically refreshes Swagger sources
func (r *Registry) refreshLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.refreshAll()
	}
}

// refreshAll refreshes all sources that need refreshing
func (r *Registry) refreshAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	for _, state := range r.sources {
		// Parse refresh interval
		interval, err := time.ParseDuration(state.source.RefreshInterval)
		if err != nil {
			interval = 1 * time.Hour // Default
		}

		// Check if refresh is needed
		if now.Sub(state.refreshTime) < interval {
			continue
		}

		log.Info("Refreshing Swagger source", "name", state.source.Name)

		// Reload the source
		if err := r.loadSource(state.source); err != nil {
			log.Error("Failed to refresh Swagger source", "name", state.source.Name, "error", err)
		}
	}
}

// loadSource loads a source (internal, caller must hold lock)
func (r *Registry) loadSource(source *Source) error {
	log.Info("Loading Swagger source", "name", source.Name, "url", source.URL)

	// Parse the Swagger document
	doc, inferredBaseURL, err := r.parser.ParseFromURL(source.URL)
	if err != nil {
		return fmt.Errorf("failed to parse swagger doc: %w", err)
	}

	// Use provided base URL or inferred one
	baseURL := source.BaseURL
	if baseURL == "" {
		baseURL = inferredBaseURL
	}

	// Get existing client or create new one
	var client *APIClient
	if state, exists := r.sources[source.ID]; exists {
		client = state.client
	} else {
		client = NewAPIClient(source.AuthConfig, baseURL, r.cfg.DefaultLimit, r.cfg.DefaultOffset)
	}

	// Create generator
	generator := NewGenerator(source, client, r.cfg.DefaultLimit, r.cfg.DefaultOffset)

	// Extract endpoints
	endpoints := r.parser.ExtractEndpoints(doc, r.cfg)

	// Limit number of endpoints
	if len(endpoints) > r.cfg.MaxEndpoints {
		endpoints = endpoints[:r.cfg.MaxEndpoints]
	}

	// Generate API tools (stored in registry for SwaggerService lookup, NOT registered to ToolRegistry)
	apiTools := generator.GenerateTools(endpoints)

	// Build toolsMap for O(1) lookup within this source
	toolsMap := make(map[string]*APITool, len(apiTools))
	for i := range apiTools {
		// Use uppercase method for case-insensitive lookup
		key := strings.ToUpper(apiTools[i].endpoint.Method) + ":" + apiTools[i].endpoint.Path
		toolsMap[key] = &apiTools[i]
		// Also add to global map
		r.allToolsMap[key] = &apiTools[i]
	}

	// Update source state
	r.sources[source.ID] = &sourceState{
		source:      source,
		client:      client,
		generator:   generator,
		tools:       apiTools,
		toolsMap:    toolsMap,
		refreshTime: time.Now(),
	}

	log.Info("Swagger source loaded", "name", source.Name, "endpoints", len(apiTools))

	return nil
}

// ReloadSource reloads a specific source
func (r *Registry) ReloadSource(sourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the source
	for i := range r.cfg.Sources {
		if r.cfg.Sources[i].ID == sourceID {
			return r.loadSource(&r.cfg.Sources[i])
		}
	}

	return fmt.Errorf("source not found: %s", sourceID)
}

// ListTools returns all API tools from all sources
func (r *Registry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := []string{}
	for _, state := range r.sources {
		for _, tool := range state.tools {
			tools = append(tools, tool.Name())
		}
	}
	return tools
}

// GetToolByMethodPath returns a tool by method and path (O(1) lookup)
func (r *Registry) GetToolByMethodPath(method, path string) (*APITool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Convert method to uppercase for case-insensitive matching
	// (stored with uppercase method names like "POST", "GET", etc.)
	key := strings.ToUpper(method) + ":" + path
	tool, exists := r.allToolsMap[key]
	return tool, exists
}

// BuildAPIToolsContext builds a description of available API tools for the system prompt
func (r *Registry) BuildAPIToolsContext() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sources) == 0 {
		return ""
	}

	lines := []string{"## Available API Endpoints", ""}
	lines = append(lines, "You have access to the following API endpoints. Use them to query and interact with business data.", "")

	// Group by source
	for _, state := range r.sources {
		lines = append(lines, fmt.Sprintf("### %s", state.source.Name))
		lines = append(lines, "")

		for _, tool := range state.tools {
			desc := tool.Description()
			if desc == "" {
				desc = fmt.Sprintf("%s %s", tool.endpoint.Method, tool.endpoint.Path)
			}
			lines = append(lines, fmt.Sprintf("- **%s**: %s", tool.Name(), desc))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
