package agentteam

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// AgentRegistry manages available agents and their capabilities
type AgentRegistry struct {
	mu      sync.RWMutex
	agents  map[string]*Agent
	created time.Time
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	registry := &AgentRegistry{
		agents:  make(map[string]*Agent),
		created: time.Now(),
	}
	registry.registerDefaultAgents()
	return registry
}

// RegisterAgent registers a new agent
func (r *AgentRegistry) RegisterAgent(agent *Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return fmt.Errorf("agent with ID %s already registered", agent.ID)
	}

	r.agents[agent.ID] = agent
	return nil
}

// GetAgent retrieves an agent by ID
func (r *AgentRegistry) GetAgent(id string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, exists := r.agents[id]
	return agent, exists
}

// ListAgents returns all registered agents
func (r *AgentRegistry) ListAgents() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// FindAgents finds agents matching capability requirements
func (r *AgentRegistry) FindAgents(criteria AgentCapabilities) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*Agent

	for _, agent := range r.agents {
		if !agent.Active {
			continue
		}

		// Check if agent is at capacity
		if agent.Capabilities.MaxConcurrent > 0 && agent.Load >= agent.Capabilities.MaxConcurrent {
			continue
		}

		// Check capabilities match
		if r.matchCapabilities(agent, criteria) {
			matches = append(matches, agent)
		}
	}

	log.Debug("AgentRegistry.FindAgents", "criteria_language", criteria.Language,
		"criteria_tools", criteria.RequiredTools, "criteria_domain", criteria.Domain,
		"matches", len(matches))

	return matches
}

// matchCapabilities checks if an agent matches the required capabilities
func (r *AgentRegistry) matchCapabilities(agent *Agent, criteria AgentCapabilities) bool {
	// Check language requirement
	if criteria.Language != "" {
		// Check if agent supports this language
		langMatched := false
		for _, lang := range agent.Capabilities.Languages {
			if lang == criteria.Language {
				langMatched = true
				break
			}
		}
		if !langMatched {
			return false
		}
	}

	// Check domain requirement
	if criteria.Domain != "" {
		domainMatched := false
		for _, domain := range agent.Capabilities.Domains {
			if domain == criteria.Domain {
				domainMatched = true
				break
			}
		}
		if !domainMatched {
			return false
		}
	}

	// Check tool requirements
	if len(criteria.RequiredTools) > 0 {
		// Check if agent has all required tools
		agentTools := make(map[string]bool)
		for _, tool := range agent.Capabilities.Tools {
			agentTools[tool] = true
		}
		for _, requiredTool := range criteria.RequiredTools {
			if !agentTools[requiredTool] {
				return false
			}
		}
	}

	return true
}

// IncrementLoad increases the load counter for an agent
func (r *AgentRegistry) IncrementLoad(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if agent.Capabilities.MaxConcurrent > 0 && agent.Load >= agent.Capabilities.MaxConcurrent {
		return fmt.Errorf("agent %s is at capacity (%d/%d)", agentID, agent.Load, agent.Capabilities.MaxConcurrent)
	}

	agent.Load++
	return nil
}

// DecrementLoad decreases the load counter for an agent
func (r *AgentRegistry) DecrementLoad(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent, exists := r.agents[agentID]; exists && agent.Load > 0 {
		agent.Load--
	}
}

// registerDefaultAgents registers default coding agents
func (r *AgentRegistry) registerDefaultAgents() {
	agents := []*Agent{
		{
			ID:          "agent-frontend",
			Name:        "Frontend Specialist",
			Description: "Specializes in frontend development",
			Capabilities: Capabilities{
				Languages:     []string{"JavaScript", "TypeScript", "HTML", "CSS"},
				Domains:       []string{"frontend", "ui", "web"},
				Tools:         []string{"React", "Vue", "Angular", "Next.js"},
				MaxConcurrent: 3,
			},
			Model:  "auto", // Will use default model
			Active: true,
			Load:   0,
		},
		{
			ID:          "agent-backend",
			Name:        "Backend Developer",
			Description: "Specializes in backend development",
			Capabilities: Capabilities{
				Languages:     []string{"Python", "Go", "Java", "Node.js"},
				Domains:       []string{"backend", "api", "database"},
				Tools:         []string{"FastAPI", "Django", "Gin", "Express"},
				MaxConcurrent: 3,
			},
			Model:  "auto",
			Active: true,
			Load:   0,
		},
		{
			ID:          "agent-database",
			Name:        "Database Specialist",
			Description: "Specializes in database design and optimization",
			Capabilities: Capabilities{
				Languages:     []string{"SQL", "Python", "Go"},
				Domains:       []string{"database", "schema", "migration"},
				Tools:         []string{"SQLAlchemy", "gorm", "migrate"},
				MaxConcurrent: 2,
			},
			Model:  "auto",
			Active: true,
			Load:   0,
		},
		{
			ID:          "agent-testing",
			Name:        "Testing Engineer",
			Description: "Specializes in writing tests and quality assurance",
			Capabilities: Capabilities{
				Languages:     []string{"Python", "JavaScript", "Go"},
				Domains:       []string{"testing", "qa", "quality"},
				Tools:         []string{"pytest", "Jest", "Cypress", "Selenium"},
				MaxConcurrent: 4,
			},
			Model:  "auto",
			Active: true,
			Load:   0,
		},
	}

	for _, agent := range agents {
		if err := r.RegisterAgent(agent); err != nil {
			log.Warn("Failed to register default agent", "agent", agent.Name, "error", err)
		} else {
			log.Info("Registered default agent", "agent", agent.ID, "name", agent.Name, "load", agent.Load, "max_concurrent", agent.Capabilities.MaxConcurrent)
		}
	}
}

// GetActiveAgentsCount returns the number of active and available agents
func (r *AgentRegistry) GetActiveAgentsCount() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	available := 0

	for _, agent := range r.agents {
		if agent.Active {
			active++
			if agent.Capabilities.MaxConcurrent == 0 || agent.Load < agent.Capabilities.MaxConcurrent {
				available++
			}
		}
	}

	return active, available
}
