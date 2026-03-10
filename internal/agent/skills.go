package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// Constants for skill management limits
const (
	// MaxAlwaysSkillsTokens is the maximum total tokens for always-loaded skills
	MaxAlwaysSkillsTokens = 2000

	// MaxSkillSizeBytes is the maximum size of a single skill file (before stripping frontmatter)
	MaxSkillSizeBytes = 50 * 1024 // 50KB

	// DefaultMaxSkills is the default maximum number of always-loaded skills
	DefaultMaxAlwaysSkills = 5
)

// SkillsLoader loads agent skills from markdown files
type SkillsLoader struct {
	mu              sync.RWMutex
	workspace       string
	workspaceSkills string
	builtinSkills   string
	cachedSkills    []SkillInfo
	cachedAt        int64
}

// SkillInfo contains information about a skill
type SkillInfo struct {
	Name   string
	Path   string
	Source string
}

// SkillMetadata contains metadata from a skill's frontmatter
type SkillMetadata struct {
	Name        string
	Description string
	Always      bool
	Metadata    string
}

// NewSkillsLoader creates a new skills loader
func NewSkillsLoader(workspace string) *SkillsLoader {
	// Get the project root directory (two levels up from internal/agent)
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	projectRoot := execDir

	// Try to find project root by looking for skills directory
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "skills")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			// Fallback to workspace
			projectRoot = ""
			break
		}
		projectRoot = parent
	}

	// Determine workspaceSkills path
	// If workspace/skills exists, use it; otherwise, use workspace directly if it looks like a skills dir
	workspaceSkills := filepath.Join(workspace, "skills")
	if _, err := os.Stat(workspaceSkills); err != nil {
		// Check if workspace itself is a skills directory
		hasSkills := false
		if entries, err := os.ReadDir(workspace); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					if _, err := os.Stat(filepath.Join(workspace, e.Name(), "SKILL.md")); err == nil {
						hasSkills = true
						break
					}
				}
			}
		}
		if hasSkills {
			workspaceSkills = workspace
		}
	}

	return &SkillsLoader{
		workspace:       workspace,
		workspaceSkills: workspaceSkills,
		builtinSkills:   filepath.Join(projectRoot, "skills"),
	}
}

// ListSkills lists all available skills
func (s *SkillsLoader) ListSkills(filterUnavailable bool) []SkillInfo {
	s.mu.RLock()
	if cached := s.getCachedSkills(); cached != nil {
		s.mu.RUnlock()
		log.Debug("[SKILLS] Using cached skills list", "count", len(cached), "cachedAt", s.cachedAt)
		return cached
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if cached := s.getCachedSkills(); cached != nil {
		log.Debug("[SKILLS] Another thread refreshed cache, using that instead")
		return cached
	}

	log.Debug("[SKILLS] Scanning for skills...", "workspaceSkills", s.workspaceSkills, "builtinSkills", s.builtinSkills)

	skills := []SkillInfo{}
	skillNames := make(map[string]bool)

	// Helper function to add skills from a directory
	addSkillsFromDir := func(skillsDir string, source string) {
		if _, err := os.Stat(skillsDir); err != nil {
			return
		}
		entries, _ := os.ReadDir(skillsDir)
		for _, entry := range entries {
			if entry.IsDir() {
				skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
				if _, err := os.Stat(skillFile); err == nil {
					// Skip if already added from another source
					if skillNames[entry.Name()] {
						continue
					}
					skillNames[entry.Name()] = true
					skills = append(skills, SkillInfo{Name: entry.Name(), Path: skillFile, Source: source})
				}
			}
		}
	}

	// Add builtin skills first (higher priority)
	addSkillsFromDir(s.builtinSkills, "builtin")

	// Add workspace skills (can override builtin)
	addSkillsFromDir(s.workspaceSkills, "workspace")

	// Update cache
	s.updateCache(skills)

	log.Info("[SKILLS] Skills cache updated",
		"total_count", len(skills),
		"builtin_count", countSkillsBySource(skills, "builtin"),
		"workspace_count", countSkillsBySource(skills, "workspace"),
		"cache_valid_seconds", 60)

	return skills
}

// countSkillsBySource counts skills by their source (builtin or workspace)
func countSkillsBySource(skills []SkillInfo, source string) int {
	count := 0
	for _, skill := range skills {
		if skill.Source == source {
			count++
		}
	}
	return count
}

// LoadSkill loads a skill by name
func (s *SkillsLoader) LoadSkill(name string) string {
	log.Debug("[SKILLS] Loading skill", "name", name)

	// Try workspace first (user skills take precedence)
	workspaceSkill := filepath.Join(s.workspaceSkills, name, "SKILL.md")
	log.Debug("[SKILLS] Checking workspace skill path", "path", workspaceSkill)
	if content, err := os.ReadFile(workspaceSkill); err == nil {
		log.Info("[SKILLS] Loaded skill from workspace", "name", name, "size", len(content))
		return string(content)
	}
	log.Debug("[SKILLS] Skill not found in workspace, trying builtin", "name", name)

	// Fall back to builtin skills
	builtinSkill := filepath.Join(s.builtinSkills, name, "SKILL.md")
	log.Debug("[SKILLS] Checking builtin skill path", "path", builtinSkill)
	if content, err := os.ReadFile(builtinSkill); err == nil {
		log.Info("[SKILLS] Loaded skill from builtin", "name", name, "size", len(content))
		return string(content)
	}

	log.Warn("[SKILLS] Skill not found", "name", name)
	return ""
}

// LoadSkillsForContext loads specific skills for inclusion in agent context
// Implements token limit to prevent context overflow
func (s *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	log.Debug("[SKILLS] LoadSkillsForContext called", "requested_skills", len(skillNames), "skill_names", skillNames)

	parts := []string{}
	totalTokens := 0
	skipped := []string{}

	for _, name := range skillNames {
		content := s.LoadSkill(name)
		if content == "" {
			log.Debug("[SKILLS] Skill not found", "name", name)
			continue
		}

		content = s.stripFrontmatter(content)
		estimatedTokens := len(content) / 4

		log.Debug("[SKILLS] Skill loaded", "name", name, "size_bytes", len(content), "estimated_tokens", estimatedTokens)

		// Check if adding this skill would exceed the limit
		if totalTokens+estimatedTokens > MaxAlwaysSkillsTokens {
			log.Warn("[SKILLS] Skill skipped due to token limit", "name", name, "tokens", estimatedTokens, "current_total", totalTokens, "limit", MaxAlwaysSkillsTokens)
			skipped = append(skipped, name)
			continue
		}

		totalTokens += estimatedTokens
		parts = append(parts, "### Skill: "+name+"\n\n"+content)
	}

	// Add warning if some skills were skipped
	if len(skipped) > 0 {
		log.Warn("[SKILLS] Skills skipped", "count", len(skipped), "names", strings.Join(skipped, ", "), "token_limit", MaxAlwaysSkillsTokens)
		warning := fmt.Sprintf("\n\n> **Note:** %d skill(s) were skipped due to size limits: %s. Use read_file tool to load them when needed.",
			len(skipped), strings.Join(skipped, ", "))
		parts = append(parts, warning)
	}

	log.Info("[SKILLS] Loaded skills for context",
		"requested", len(skillNames),
		"loaded", len(parts),
		"skipped", len(skipped),
		"total_tokens", totalTokens)

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildSkillsSummary builds a summary of all skills
func (s *SkillsLoader) BuildSkillsSummary() string {
	allSkills := s.ListSkills(false)
	if len(allSkills) == 0 {
		return ""
	}

	lines := []string{"<skills>"}
	for _, skill := range allSkills {
		name := escapeXML(skill.Name)
		lines = append(lines, fmt.Sprintf(`  <skill available="true">`))
		lines = append(lines, "    <name>"+name+"</name>")
		lines = append(lines, "    <location>"+skill.Path+"</location>")
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")
	return strings.Join(lines, "\n")
}

// GetSkillMetadata gets metadata from a skill's frontmatter
func (s *SkillsLoader) GetSkillMetadata(name string) *SkillMetadata {
	content := s.LoadSkill(name)
	if content == "" {
		return nil
	}

	if strings.HasPrefix(content, "---") {
		// Use (?s) to make . match newlines for multi-line frontmatter
		re := regexp.MustCompile(`(?s)^---\n(.*?)\n---`)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			meta := &SkillMetadata{}
			lines := strings.Split(matches[1], "\n")
			for _, line := range lines {
				if strings.Contains(line, ":") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(strings.Trim(parts[1], `"'`))
						switch key {
						case "name":
							meta.Name = value
						case "description":
							meta.Description = value
						case "always":
							meta.Always = value == "true"
						case "metadata":
							meta.Metadata = value
						}
					}
				}
			}
			return meta
		}
	}
	return nil
}

// GetAlwaysSkills returns skills marked as always=true
// Limits the number of always-loaded skills to prevent context overflow
func (s *SkillsLoader) GetAlwaysSkills() []string {
	result := []string{}
	tokenCount := 0

	for _, skill := range s.ListSkills(true) {
		meta := s.GetSkillMetadata(skill.Name)
		if meta != nil && meta.Always {
			// Check token limit
			skillTokens := s.GetSkillTokenEstimate(skill.Name)
			if tokenCount+skillTokens > MaxAlwaysSkillsTokens {
				continue
			}
			// Limit number of skills
			if len(result) >= DefaultMaxAlwaysSkills {
				continue
			}
			result = append(result, skill.Name)
			tokenCount += skillTokens
		}
	}
	return result
}

func (s *SkillsLoader) getSkillMeta(name string) map[string]interface{} {
	meta := s.GetSkillMetadata(name)
	if meta == nil {
		return make(map[string]interface{})
	}
	if meta.Metadata != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(meta.Metadata), &data); err == nil {
			if wukongbot, ok := data["wukongbot"].(map[string]interface{}); ok {
				return wukongbot
			}
		}
	}
	return make(map[string]interface{})
}

func (s *SkillsLoader) getSkillDescription(name string) string {
	meta := s.GetSkillMetadata(name)
	if meta != nil && meta.Description != "" {
		return meta.Description
	}
	return name
}

func (s *SkillsLoader) checkRequirements(skillMeta map[string]interface{}) bool {
	requires, ok := skillMeta["requires"].(map[string]interface{})
	if !ok {
		return true
	}
	if bins, ok := requires["bins"].([]interface{}); ok {
		for _, b := range bins {
			if binStr, ok := b.(string); ok {
				if _, err := exec.LookPath(binStr); err != nil {
					return false
				}
			}
		}
	}
	if envs, ok := requires["env"].([]interface{}); ok {
		for _, e := range envs {
			if envStr, ok := e.(string); ok {
				if os.Getenv(envStr) == "" {
					return false
				}
			}
		}
	}
	return true
}

func (s *SkillsLoader) stripFrontmatter(content string) string {
	if strings.HasPrefix(content, "---") {
		// Use (?s) to make . match newlines for multi-line frontmatter
		re := regexp.MustCompile(`(?s)^---\n.*?\n---\n`)
		matched := re.FindString(content)
		if matched != "" {
			return strings.TrimSpace(content[len(matched):])
		}
	}
	return content
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// GetSkillSize returns the size of a skill in bytes
func (s *SkillsLoader) GetSkillSize(name string) int64 {
	content := s.LoadSkill(name)
	if content == "" {
		return 0
	}
	return int64(len(content))
}

// IsSkillTooLarge returns true if the skill exceeds the size limit
func (s *SkillsLoader) IsSkillTooLarge(name string) bool {
	return s.GetSkillSize(name) > MaxSkillSizeBytes
}

// GetSkillTokenEstimate estimates the number of tokens in a skill
// Using rough estimate: 4 characters per token
func (s *SkillsLoader) GetSkillTokenEstimate(name string) int {
	content := s.LoadSkill(name)
	if content == "" {
		return 0
	}
	return len(content) / 4
}

// Reload clears the cache and re-scans all skills directories
func (s *SkillsLoader) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedSkills = nil
	s.cachedAt = 0
}

// getCachedSkills returns cached skills list or rebuilds it
func (s *SkillsLoader) getCachedSkills() []SkillInfo {
	// Check if cache is valid (less than 1 minute old)
	if s.cachedSkills != nil && s.cachedAt > 0 {
		if time.Now().Unix()-s.cachedAt < 60 {
			return s.cachedSkills
		}
	}
	return nil
}

// updateCache updates the skills cache
func (s *SkillsLoader) updateCache(skills []SkillInfo) {
	s.cachedSkills = skills
	s.cachedAt = time.Now().Unix()
}
