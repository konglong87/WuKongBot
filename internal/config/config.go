package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/konglong87/wukongbot/internal/hooks"
)

// WhatsAppConfig holds WhatsApp channel configuration
type WhatsAppConfig struct {
	Enabled   bool     `yaml:"enabled"`
	BridgeURL string   `yaml:"bridge_url"`
	AllowFrom []string `yaml:"allow_from"`
}

// TelegramConfig holds Telegram channel configuration
type TelegramConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Token     string   `yaml:"token"`
	AllowFrom []string `yaml:"allow_from"`
	Proxy     string   `yaml:"proxy,omitempty"`
}

// FeishuConfig holds Feishu/Lark channel configuration
type FeishuConfig struct {
	Enabled           bool     `yaml:"enabled"`
	AppID             string   `yaml:"app_id"`
	AppSecret         string   `yaml:"app_secret"`
	EncryptKey        string   `yaml:"encrypt_key,omitempty"`
	VerificationToken string   `yaml:"verification_token,omitempty"`
	AllowFrom         []string `yaml:"allow_from"`
	ConnectionMode    string   `yaml:"connection_mode,omitempty"` // "websocket" or "webhook", default is "websocket"
}

// ChannelsConfig holds all channel configurations
type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	Telegram TelegramConfig `yaml:"telegram"`
	Feishu   FeishuConfig   `yaml:"feishu"`
}

// ClaudeCodeConfig holds Claude Code CLI integration configuration
type ClaudeCodeConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Workspace      string   `yaml:"workspace"`
	SessionPrefix  string   `yaml:"session_prefix"`
	SessionTimeout int      `yaml:"session_timeout"` // in seconds
	AutoCleanup    bool     `yaml:"auto_cleanup"`
	MaxSessions    int      `yaml:"max_sessions"`
	ClaudeCommand  []string `yaml:"claude_command"` // CLI command, e.g., ["claude", "code"] or ["opencode", "code"]
}

// TmuxConfig holds tmux session management configuration
type TmuxConfig struct {
	Enabled         bool   `yaml:"enabled"`          // Enable tmux session management
	SocketDir       string `yaml:"socket_dir"`       // Socket directory path
	DefaultInterval int64  `yaml:"default_interval"` // Default monitoring interval (seconds)
	MaxSessions     int    `yaml:"max_sessions"`     // Max sessions per user
	OutputLines     int    `yaml:"output_lines"`     // Lines to capture per monitoring
	WaitMs          int    `yaml:"wait_ms"`          // Wait time before capturing output (ms)
	SessionTimeout  int64  `yaml:"session_timeout"`  // Session timeout (seconds)
	AutoRestart     bool   `yaml:"auto_restart"`     // Auto-restart crashed sessions
	MaxRestarts     int    `yaml:"max_restarts"`     // Max restart attempts
}

// AgentDefaults holds default agent configuration
type AgentDefaults struct {
	Workspace             string  `yaml:"workspace"`
	Model                 string  `yaml:"model"`
	MaxTokens             int     `yaml:"max_tokens"`
	Temperature           float64 `yaml:"temperature"`
	MaxToolIterations     int     `yaml:"max_tool_iterations"`
	MaxHistoryMessages    int     `yaml:"max_history_messages"`    // Maximum history messages to include (0 = no history)
	UseHistoryMessages    bool    `yaml:"use_history_messages"`    // Whether to use history messages (false for real-time data queries)
	HistoryTimeoutSeconds int     `yaml:"history_timeout_seconds"` // History message timeout in seconds (0 = no limit)
	ErrorResponse         string  `yaml:"error_response"`          // Default error response when LLM returns empty content
}

// IdentityConfig holds agent identity configuration
type IdentityConfig struct {
	Name     string `yaml:"name"`
	Title    string `yaml:"title"`
	Prompt   string `yaml:"prompt"`
	Tools    string `yaml:"tools"`
	Behavior string `yaml:"behavior"`
	Soul     string `yaml:"soul"` // 机器人的性格，如古灵精怪、踏实稳妥、活泼开朗乐观向上等
}

// AgentsConfig holds agent configuration
type AgentsConfig struct {
	Defaults AgentDefaults `yaml:"defaults"`
}

// ProviderConfig holds LLM provider configuration
type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	APIBase string `yaml:"api_base,omitempty"`
}

// ProvidersConfig holds all LLM provider configurations
type ProvidersConfig struct {
	Anthropic  ProviderConfig `yaml:"anthropic"`
	OpenAI     ProviderConfig `yaml:"openai"`
	OpenRouter ProviderConfig `yaml:"openrouter"`
	DeepSeek   ProviderConfig `yaml:"deepseek"`
	Groq       ProviderConfig `yaml:"groq"`
	Zhipu      ProviderConfig `yaml:"zhipu"`
	VLLM       ProviderConfig `yaml:"vllm"`
	Gemini     ProviderConfig `yaml:"gemini"`
	Qwen       ProviderConfig `yaml:"qwen"`
}

// GatewayConfig holds gateway/server configuration
type GatewayConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

// WebSearchConfig holds web search tool configuration
type WebSearchConfig struct {
	Provider   string `yaml:"provider"` // "brave" or "duckduckgo"
	APIKey     string `yaml:"api_key"`
	MaxResults int    `yaml:"max_results"`
}

// WebToolsConfig holds web tools configuration
type WebToolsConfig struct {
	Search WebSearchConfig `yaml:"search"`
}

// ExecToolConfig holds shell exec tool configuration
type ExecToolConfig struct {
	Timeout             int  `yaml:"timeout"`
	RestrictToWorkspace bool `yaml:"restrict_to_workspace"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type     string `yaml:"type"`     // "sqlite" or "mysql"
	Path     string `yaml:"path"`     // SQLite database path
	Host     string `yaml:"host"`     // MySQL host
	Port     int    `yaml:"port"`     // MySQL port
	Database string `yaml:"database"` // MySQL database name
	Username string `yaml:"username"` // MySQL username
	Password string `yaml:"password"` // MySQL password
}

// ToolsConfig holds tools configuration
type ToolsConfig struct {
	Web   WebToolsConfig   `yaml:"web"`
	Exec  ExecToolConfig   `yaml:"exec"`
	Image ImageToolsConfig `yaml:"image"`
	Hooks hooks.Config     `yaml:"hooks"`
}

// ImageToolsConfig holds image tools configuration
type ImageToolsConfig struct {
	Generation ImageGenerationConfig `yaml:"generation"`
	Analysis   ImageAnalysisConfig   `yaml:"analysis"`
}

// ImageGenerationConfig holds image generation tool configuration
type ImageGenerationConfig struct {
	APIKey  string            `yaml:"api_key"`
	APIBase string            `yaml:"api_base"`
	Model   string            `yaml:"model"`
	Models  map[string]string `yaml:"models"` // Model-specific API bases
}

// ImageAnalysisConfig holds image analysis tool configuration
type ImageAnalysisConfig struct {
	APIKey  string `yaml:"api_key"`
	APIBase string `yaml:"api_base"`
	Model   string `yaml:"model"`
}

// CodeDevConfig holds code development tool configuration
type CodeDevConfig struct {
	Enabled   bool                             `yaml:"enabled"`
	Timeout   int                              `yaml:"timeout"` // seconds, default 300 (5 min)
	Executors map[string]CodeDevExecutorConfig `yaml:"executors"`
}

// CodeDevExecutorConfig holds configuration for a code dev executor
type CodeDevExecutorConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Command  string `yaml:"command"`
	Template string `yaml:"template"`
}

// SwaggerConfig holds Swagger/OpenAPI integration configuration
type SwaggerConfig struct {
	Sources       []SwaggerSource `yaml:"sources"`
	MaxEndpoints  int             `yaml:"max_endpoints"`
	IncludeTags   []string        `yaml:"include_tags"`
	ExcludeTags   []string        `yaml:"exclude_tags"`
	DefaultLimit  int             `yaml:"default_limit"`
	DefaultOffset int             `yaml:"default_offset"`
}

// SwaggerSource defines a single API source
type SwaggerSource struct {
	ID              string     `yaml:"id"`
	Name            string     `yaml:"name"`
	URL             string     `yaml:"url"`
	BaseURL         string     `yaml:"base_url"`
	AuthConfig      AuthConfig `yaml:"auth"`
	Enabled         bool       `yaml:"enabled"`
	RefreshInterval string     `yaml:"refresh_interval"`
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Type         string            `yaml:"type"`
	Token        string            `yaml:"token"`
	Username     string            `yaml:"username"`
	Password     string            `yaml:"password"`
	Headers      map[string]string `yaml:"headers"`
	ClientID     string            `yaml:"client_id"`
	ClientSecret string            `yaml:"client_secret"`
	TokenURL     string            `yaml:"token_url"`

	// Token refresh configuration
	RefreshURL string `yaml:"refresh_url"` // Token refresh endpoint, e.g., "/base/captchaLogin"
	CaptchaURL string `yaml:"captcha_url"` // Captcha fetch endpoint, e.g., "/base/getToken"
	Phone      string `yaml:"phone"`       // Phone number for login
}

// Config is the root configuration for wukongbot
type Config struct {
	Agents     AgentsConfig     `yaml:"agents"`
	Channels   ChannelsConfig   `yaml:"channels"`
	Providers  ProvidersConfig  `yaml:"providers"`
	Gateway    GatewayConfig    `yaml:"gateway"`
	Tools      ToolsConfig      `yaml:"tools"`
	Database   DatabaseConfig   `yaml:"database"`
	Identity   IdentityConfig   `yaml:"identity"`
	Swagger    SwaggerConfig    `yaml:"swagger"`
	CodeDev    CodeDevConfig    `yaml:"code_dev"`
	AgentTeam  AgentTeamConfig  `yaml:"agent_team"`
	ClaudeCode ClaudeCodeConfig `yaml:"claude_code"`
	Tmux       TmuxConfig       `yaml:"tmux"`
}

// AgentTeamConfig holds agent team configuration
type AgentTeamConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"` // LLM model for decomposition and coordination
}

// WorkspacePath returns the expanded workspace path
func (c *Config) WorkspacePath() string {
	path := c.Agents.Defaults.Workspace
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return path
}

// ResolveImageAPIKeys resolves image tool API keys, falling back to providers.qwen.api_key if empty
func (c *Config) ResolveImageAPIKeys() {
	// If generation API key is empty, use qwen API key
	if c.Tools.Image.Generation.APIKey == "" {
		c.Tools.Image.Generation.APIKey = c.Providers.Qwen.APIKey
	}

	// If analysis API key is empty, use qwen API key
	if c.Tools.Image.Analysis.APIKey == "" {
		c.Tools.Image.Analysis.APIKey = c.Providers.Qwen.APIKey
	}
}

// GetAPIKey returns the API key in priority order
func (c *Config) GetAPIKey() string {
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.DeepSeek.APIKey != "" {
		return c.Providers.DeepSeek.APIKey
	}
	if c.Providers.Anthropic.APIKey != "" {
		return c.Providers.Anthropic.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	return ""
}

// GetAPIBase returns the API base URL if using OpenRouter, Zhipu, vLLM, DeepSeek or OpenAI
func (c *Config) GetAPIBase() string {
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.DeepSeek.APIKey != "" {
		if c.Providers.DeepSeek.APIBase != "" {
			return c.Providers.DeepSeek.APIBase
		}
		return "https://api.deepseek.com"
	}
	if c.Providers.Zhipu.APIKey != "" && c.Providers.Zhipu.APIBase != "" {
		return c.Providers.Zhipu.APIBase
	}
	if c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	if c.Providers.OpenAI.APIBase != "" {
		return c.Providers.OpenAI.APIBase
	}
	return ""
}
