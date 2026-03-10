package agent

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
)

// ToolPolicy manages tool access permissions for subagents
type ToolPolicy struct {
	Mode      string         `yaml:"mode"` // "default", "allow_only", "deny_only"
	AllowList []string       `yaml:"allow_list"`
	DenyList  []string       `yaml:"deny_list"`
	Resources ResourcePolicy `yaml:"resources"`

	// Internal compiled structures
	allowMap map[string]bool
	denyMap  map[string]bool
}

// ResourcePolicy defines resource usage limits
type ResourcePolicy struct {
	MaxConcurrent int    `yaml:"max_concurrent"`  // Max concurrent subagents
	Timeout       int    `yaml:"timeout_seconds"` // Max execution time in seconds
	MemoryLimit   string `yaml:"memory_limit"`    // Memory limit (e.g., "512M")
}

// DefaultDenyTools are the tools that should be denied by default for subagents
var DefaultDenyTools = []string{
	"spawn",           // Prevent recursive spawning
	"sessions_spawn",  // Prevent session management
	"sessions_list",   // Prevent session enumeration
	"sessions_delete", // Prevent session deletion
	"gateway",         // Prevent direct gateway access
	"cron",            // Prevent cron job manipulation
}

// DefaultPolicy returns the default policy for subagents
func DefaultPolicy() *ToolPolicy {
	return &ToolPolicy{
		Mode:      "allow_only",
		AllowList: []string{"read_file", "write_file", "edit_file", "list_dir", "exec", "web_search", "web_fetch"},
		DenyList:  DefaultDenyTools,
		Resources: ResourcePolicy{
			MaxConcurrent: 5,
			Timeout:       300, // 5 minutes
			MemoryLimit:   "512M",
		},
	}
}

// ParsePolicy parses a policy from YAML string
func ParsePolicy(policyStr string) (*ToolPolicy, error) {
	policy := DefaultPolicy()
	if err := yaml.Unmarshal([]byte(policyStr), policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}
	policy.compile()
	return policy, nil
}

// Compile compiles the policy maps for faster lookups
func (p *ToolPolicy) compile() {
	p.allowMap = make(map[string]bool)
	for _, tool := range p.AllowList {
		p.allowMap[tool] = true
	}

	p.denyMap = make(map[string]bool)
	for _, tool := range p.DenyList {
		p.denyMap[tool] = true
	}
}

// IsToolAllowed checks if a tool is allowed by this policy
func (p *ToolPolicy) IsToolAllowed(toolName string) bool {
	// First check deny list (has priority)
	if p.denyMap != nil {
		// Check exact match
		if p.denyMap[toolName] {
			return false
		}

		// Check wildcard patterns
		for denyPattern := range p.denyMap {
			if strings.HasSuffix(denyPattern, "*") {
				prefix := strings.TrimSuffix(denyPattern, "*")
				if strings.HasPrefix(toolName, prefix) {
					return false
				}
			}
		}
	}

	// Then check mode
	switch p.Mode {
	case "deny_only":
		// Only deny list is checked, everything else is allowed
		return true
	case "allow_only":
		// Allow list is checked, everything else is denied
		return p.allowMap[toolName]
	case "default":
		// Default behavior: allow list + not in deny list
		if p.allowMap[toolName] {
			return true
		}
		// If not explicitly allowed, it's denied
		return false
	default:
		return true // Default to allow if mode is unknown
	}
}

// ValidateResourceUsage checks if the resource usage is within limits
func (p *ToolPolicy) ValidateResourceUsage(currentConcurrent int) bool {
	if p.Resources.MaxConcurrent <= 0 {
		return true // No limit
	}
	return currentConcurrent < p.Resources.MaxConcurrent
}

// GetTimeout returns the timeout in seconds
func (p *ToolPolicy) GetTimeout() int {
	if p.Resources.Timeout <= 0 {
		return 300 // Default 5 minutes
	}
	return p.Resources.Timeout
}

// PolicyRegistry manages multiple policies
type PolicyRegistry struct {
	policies      map[string]*ToolPolicy
	defaultPolicy *ToolPolicy
}

// NewPolicyRegistry creates a new policy registry
func NewPolicyRegistry() *PolicyRegistry {
	registry := &PolicyRegistry{
		policies: make(map[string]*ToolPolicy),
	}
	registry.defaultPolicy = DefaultPolicy()
	registry.defaultPolicy.compile()
	return registry
}

// RegisterPolicy registers a policy
func (r *PolicyRegistry) RegisterPolicy(name string, policy *ToolPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}
	policy.compile()
	r.policies[name] = policy
	return nil
}

// GetPolicy retrieves a policy by name, returns default if not found
func (r *PolicyRegistry) GetPolicy(name string) *ToolPolicy {
	if policy, ok := r.policies[name]; ok {
		return policy
	}
	return r.defaultPolicy
}

// ListPolicies returns all registered policy names
func (r *PolicyRegistry) ListPolicies() []string {
	names := make([]string, 0, len(r.policies)+1)
	names = append(names, "default")
	for name := range r.policies {
		names = append(names, name)
	}
	return names
}
