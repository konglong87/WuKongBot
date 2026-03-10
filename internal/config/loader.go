package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = ".wukongbot/config.yaml"
	envPrefix         = "wukongbot_"
	envDelimiter      = "__"
)

// Load loads configuration from file and applies environment variable overrides
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = defaultConfigPath
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func applyEnvOverrides(cfg *Config) {
	// Collect all env vars with wukongbot_ prefix
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, envPrefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimPrefix(parts[0], envPrefix)
			value := parts[1]

			// Handle nested keys using __ delimiter
			applyEnvOverride(cfg, key, value)
		}
	}
}

// applyEnvOverride applies a single environment variable override
func applyEnvOverride(cfg *Config, key, value string) {
	path := strings.Split(key, envDelimiter)

	switch {
	case len(path) >= 3:
		// Handle nested structures like AGENTS__DEFAULTS__MODEL
		prefix := strings.ToLower(path[0])
		section := strings.ToLower(path[1])
		field := strings.ToLower(path[2])

		switch prefix {
		case "agents":
			if section == "defaults" {
				switch field {
				case "workspace":
					cfg.Agents.Defaults.Workspace = value
				case "model":
					cfg.Agents.Defaults.Model = value
				case "max_tokens":
					cfg.Agents.Defaults.MaxTokens = parseInt(value)
				case "temperature":
					cfg.Agents.Defaults.Temperature = parseFloat(value)
				case "max_tool_iterations":
					cfg.Agents.Defaults.MaxToolIterations = parseInt(value)
				}
			}
		case "providers":
			switch section {
			case "anthropic":
				if field == "api_key" {
					cfg.Providers.Anthropic.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.Anthropic.APIBase = value
				}
			case "openai":
				if field == "api_key" {
					cfg.Providers.OpenAI.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.OpenAI.APIBase = value
				}
			case "openrouter":
				if field == "api_key" {
					cfg.Providers.OpenRouter.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.OpenRouter.APIBase = value
				}
			case "deepseek":
				if field == "api_key" {
					cfg.Providers.DeepSeek.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.DeepSeek.APIBase = value
				}
			case "groq":
				if field == "api_key" {
					cfg.Providers.Groq.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.Groq.APIBase = value
				}
			case "zhipu":
				if field == "api_key" {
					cfg.Providers.Zhipu.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.Zhipu.APIBase = value
				}
			case "vllm":
				if field == "api_key" {
					cfg.Providers.VLLM.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.VLLM.APIBase = value
				}
			case "gemini":
				if field == "api_key" {
					cfg.Providers.Gemini.APIKey = value
				} else if field == "api_base" {
					cfg.Providers.Gemini.APIBase = value
				}
			}
		case "channels":
			switch section {
			case "whatsapp":
				if field == "enabled" {
					cfg.Channels.WhatsApp.Enabled = parseBool(value)
				} else if field == "bridge_url" {
					cfg.Channels.WhatsApp.BridgeURL = value
				} else if field == "allow_from" {
					cfg.Channels.WhatsApp.AllowFrom = parseList(value)
				}
			case "telegram":
				if field == "enabled" {
					cfg.Channels.Telegram.Enabled = parseBool(value)
				} else if field == "token" {
					cfg.Channels.Telegram.Token = value
				} else if field == "proxy" {
					cfg.Channels.Telegram.Proxy = value
				} else if field == "allow_from" {
					cfg.Channels.Telegram.AllowFrom = parseList(value)
				}
			case "feishu":
				if field == "enabled" {
					cfg.Channels.Feishu.Enabled = parseBool(value)
				} else if field == "app_id" {
					cfg.Channels.Feishu.AppID = value
				} else if field == "app_secret" {
					cfg.Channels.Feishu.AppSecret = value
				} else if field == "encrypt_key" {
					cfg.Channels.Feishu.EncryptKey = value
				} else if field == "verification_token" {
					cfg.Channels.Feishu.VerificationToken = value
				} else if field == "allow_from" {
					cfg.Channels.Feishu.AllowFrom = parseList(value)
				}
			}
		case "tools":
			switch section {
			case "web":
				if field == "api_key" {
					cfg.Tools.Web.Search.APIKey = value
				} else if field == "max_results" {
					cfg.Tools.Web.Search.MaxResults = parseInt(value)
				}
			case "exec":
				if field == "timeout" {
					cfg.Tools.Exec.Timeout = parseInt(value)
				} else if field == "restrict_to_workspace" {
					cfg.Tools.Exec.RestrictToWorkspace = parseBool(value)
				}
			}
		case "gateway":
			if field == "host" {
				cfg.Gateway.Host = value
			} else if field == "port" {
				cfg.Gateway.Port = parseInt(value)
			}
		}
	}
}

// Helper functions for parsing env var values
func parseInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func parseBool(s string) bool {
	return strings.ToLower(s) == "true" || s == "1"
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	// Simple comma-separated list parsing
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
