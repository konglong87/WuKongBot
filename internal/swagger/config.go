package swagger

// Type aliases to use config package types
type SwaggerConfig = Config
type SwaggerSource = Source
type AuthConfig = Auth

// Config holds the swagger configuration (mirrors config.SwaggerConfig)
type Config struct {
	Sources       []Source `yaml:"sources"`
	MaxEndpoints  int      `yaml:"max_endpoints"`
	IncludeTags   []string `yaml:"include_tags"`
	ExcludeTags   []string `yaml:"exclude_tags"`
	DefaultLimit  int      `yaml:"default_limit"`
	DefaultOffset int      `yaml:"default_offset"`
}

// Source defines a single API source (mirrors config.SwaggerSource)
type Source struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	URL             string `yaml:"url"`
	BaseURL         string `yaml:"base_url"`
	AuthConfig      Auth   `yaml:"auth"`
	Enabled         bool   `yaml:"enabled"`
	RefreshInterval string `yaml:"refresh_interval"`
}

// Auth defines authentication configuration (mirrors config.AuthConfig)
type Auth struct {
	Type         string            `yaml:"type"`
	Token        string            `yaml:"token"`
	Username     string            `yaml:"username"`
	Password     string            `yaml:"password"`
	Headers      map[string]string `yaml:"headers"`
	ClientID     string            `yaml:"client_id"`
	ClientSecret string            `yaml:"client_secret"`
	TokenURL     string            `yaml:"token_url"`

	// Token refresh configuration
	RefreshURL    string `yaml:"refresh_url"` // Token refresh endpoint, e.g., "/base/captchaLogin"
	CaptchaURL    string `yaml:"captcha_url"` // Captcha fetch endpoint, e.g., "/base/getToken"
	Phone         string `yaml:"phone"`       // Phone number for login
	TokenExpireAt int64  `yaml:"-"`           // Token expiration timestamp (milliseconds)
	TokenPath     string `yaml:"token_path"`  // JSON path to token in response, e.g., "$.data.token"
}

// DefaultConfig returns the default swagger configuration
func DefaultConfig() *Config {
	return &Config{
		MaxEndpoints:  50,
		DefaultLimit:  1000,
		DefaultOffset: 0,
	}
}
