package swagger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// APIClient handles making HTTP requests to APIs
type APIClient struct {
	httpClient    *http.Client
	authConfig    Auth
	authConfigMu  sync.RWMutex // Protect authConfig for concurrent token refresh
	baseURL       string
	defaultLimit  int
	defaultOffset int
}

// NewAPIClient creates a new API client
func NewAPIClient(authConfig Auth, baseURL string, defaultLimit, defaultOffset int) *APIClient {
	return &APIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &retryableTransport{
				Transport:  http.DefaultTransport,
				MaxRetries: 3,
			},
		},
		authConfig:    authConfig,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		defaultLimit:  defaultLimit,
		defaultOffset: defaultOffset,
	}
}

// ExecuteRequest executes an HTTP request with the given parameters
func (c *APIClient) ExecuteRequest(ctx context.Context, method string, path string, pathParams, queryParams, bodyParams map[string]interface{}) (string, error) {
	// First attempt
	result, needsRefresh, err := c.executeRequest(ctx, method, path, pathParams, queryParams, bodyParams)
	if err != nil {
		return "", err
	}

	// Check if token refresh is needed
	if needsRefresh {
		log.Info("Token expired, attempting to refresh")
		if err := c.refreshToken(ctx); err != nil {
			log.Error("Failed to refresh token", "error", err)
			return result, nil // Return original error even if refresh failed
		}
		log.Info("Token refreshed successfully, retrying request")
		// Retry with new token
		result, _, err = c.executeRequest(ctx, method, path, pathParams, queryParams, bodyParams)
	}

	return result, err
}

// executeRequest performs a single HTTP request
// Returns (response, needsTokenRefresh, error)
func (c *APIClient) executeRequest(ctx context.Context, method string, path string, pathParams, queryParams, bodyParams map[string]interface{}) (string, bool, error) {
	// Normalize HTTP method to uppercase (HTTP/1.1 requires uppercase methods)
	normalizedMethod := strings.ToUpper(method)

	// Build the URL
	fullPath := path
	for key, value := range pathParams {
		fullPath = strings.ReplaceAll(fullPath, "{"+key+"}", fmt.Sprintf("%v", value))
	}

	fullPath = strings.TrimPrefix(fullPath, "/")
	requestURL := c.baseURL + "/" + fullPath

	// Build query string
	if len(queryParams) > 0 {
		queryValues := url.Values{}
		for key, value := range queryParams {
			queryValues.Add(key, fmt.Sprintf("%v", value))
		}
		requestURL += "?" + queryValues.Encode()
	}

	// Prepare request body
	var bodyReader io.Reader
	if bodyParams != nil && len(bodyParams) > 0 {
		bodyJSON, err := json.Marshal(bodyParams)
		if err != nil {
			return "", false, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyJSON))
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, normalizedMethod, requestURL, bodyReader)
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply authentication
	if err := c.applyAuth(req); err != nil {
		return "", false, fmt.Errorf("failed to apply auth: %w", err)
	}

	// Set custom headers
	c.authConfigMu.RLock()
	headers := c.authConfig.Headers
	c.authConfigMu.RUnlock()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Log request details for debugging
	log.Debug("Sending API request", "method", normalizedMethod, "url", requestURL,
		"auth_header", req.Header.Get("Authorization"), "headers", req.Header)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	var responseJSON interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseJSON); err != nil {
		// If JSON parsing fails, return raw text
		body := make([]byte, 0, 1024)
		resp.Body.Read(body)
		return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)), false, nil
	}

	// Check for token expired error (code: 7)
	needsRefresh := false
	if responseMap, ok := responseJSON.(map[string]interface{}); ok {
		if code, ok := responseMap["code"].(float64); ok && code == 7 {
			needsRefresh = true
			log.Info("Token expired detected", "code", code, "msg", responseMap["msg"])
		}
	}

	// Format JSON response
	formatted, err := json.MarshalIndent(responseJSON, "", "  ")
	if err != nil {
		return "", false, fmt.Errorf("failed to format response: %w", err)
	}

	log.Debug("API request completed", "method", normalizedMethod, "url", requestURL, "status", resp.StatusCode, "needs_refresh", needsRefresh)

	return string(formatted), needsRefresh, nil
}

// applyAuth applies authentication to the request
func (c *APIClient) applyAuth(req *http.Request) error {
	authType := strings.ToLower(c.authConfig.Type)
	log.Debug("Applying auth", "type", authType, "has_token", c.authConfig.Token != "", "token_length", len(c.authConfig.Token))

	switch authType {
	case "bearer":
		if c.authConfig.Token != "" {
			authHeader := "Bearer " + c.authConfig.Token
			req.Header.Set("Authorization", authHeader)
			log.Debug("Set Authorization header", "header_length", len(authHeader))
		} else {
			log.Warn("Bearer auth requested but no token provided")
		}
	case "basic":
		if c.authConfig.Username != "" {
			req.SetBasicAuth(c.authConfig.Username, c.authConfig.Password)
		}
	case "apikey", "api_key":
		if c.authConfig.Token != "" {
			// Try common header names
			req.Header.Set("X-API-Key", c.authConfig.Token)
			req.Header.Set("Authorization", "Bearer "+c.authConfig.Token)
		}
	case "xtoken", "x-token":
		// Use x-token header for authentication
		if c.authConfig.Token != "" {
			req.Header.Set("x-token", c.authConfig.Token)
			log.Debug("Set x-token header", "token_length", len(c.authConfig.Token))
		} else {
			log.Warn("x-token auth requested but no token provided")
		}
	case "oauth2":
		// TODO: Implement OAuth2 flow
		return fmt.Errorf("OAuth2 authentication not yet implemented")
	case "", "none":
		// No authentication
		log.Debug("No authentication configured")
	default:
		return fmt.Errorf("unknown auth type: %s", c.authConfig.Type)
	}
	return nil
}

// refreshToken refreshes the authentication token using captcha login flow
func (c *APIClient) refreshToken(ctx context.Context) error {
	c.authConfigMu.Lock()
	defer c.authConfigMu.Unlock()

	// Debug: log auth config
	log.Debug("Token refresh - auth config", "refresh_url", c.authConfig.RefreshURL, "captcha_url", c.authConfig.CaptchaURL, "phone", c.authConfig.Phone)

	// Check if refresh configuration is available
	if c.authConfig.RefreshURL == "" {
		return fmt.Errorf("token refresh not configured (missing refresh_url)")
	}
	if c.authConfig.CaptchaURL == "" {
		return fmt.Errorf("token refresh not configured (missing captcha_url)")
	}
	if c.authConfig.Phone == "" {
		return fmt.Errorf("token refresh not configured (missing phone)")
	}

	// Step 1: Get captcha from /base/getToken
	captcha, err := c.getCaptcha(ctx)
	if err != nil {
		return fmt.Errorf("failed to get captcha: %w", err)
	}
	log.Info("Got captcha", "captcha", captcha)

	// Step 2: Login with captcha
	newToken, expiresAt, err := c.loginWithCaptcha(ctx, captcha)
	if err != nil {
		return fmt.Errorf("failed to login with captcha: %w", err)
	}

	// Step 3: Update token
	c.authConfig.Token = newToken
	c.authConfig.TokenExpireAt = expiresAt
	log.Info("Token refreshed successfully", "expires_at", expiresAt, "token_length", len(newToken))

	return nil
}

// getCaptcha fetches captcha from the captcha endpoint
func (c *APIClient) getCaptcha(ctx context.Context) (string, error) {
	captchaURL := strings.TrimPrefix(c.authConfig.CaptchaURL, "/")
	fullURL := c.baseURL + "/" + captchaURL

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create captcha request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch captcha: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("captcha API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode captcha response: %w", err)
	}

	// Extract captcha from response (assuming it's in the data field or similar)
	// Common patterns: {"code": 0, "data": {"captcha": "123456"}}, {"code": 0, "data": "123456"}
	var captcha string
	if data, ok := result["data"]; ok {
		if str, ok := data.(string); ok {
			captcha = str
		} else if dataMap, ok := data.(map[string]interface{}); ok {
			// Try common field names
			if val, ok := dataMap["captcha"].(string); ok {
				captcha = val
			} else if val, ok := dataMap["code"].(string); ok {
				captcha = val
			} else if val, ok := dataMap["token"].(string); ok {
				captcha = val
			}
		}
	}

	if captcha == "" {
		log.Warn("Could not extract captcha from response", "response", result)
		// Return a default 6-digit captcha as fallback (may not work in production)
		return "123456", nil
	}

	return captcha, nil
}

// loginWithCaptcha performs login with phone and captcha
func (c *APIClient) loginWithCaptcha(ctx context.Context, captcha string) (string, int64, error) {
	refreshURL := strings.TrimPrefix(c.authConfig.RefreshURL, "/")
	fullURL := c.baseURL + "/" + refreshURL

	// Build login request body
	loginBody := map[string]interface{}{
		"phone":   c.authConfig.Phone,
		"captcha": captcha,
	}

	bodyJSON, err := json.Marshal(loginBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal login body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("login API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("failed to decode login response: %w", err)
	}

	// Check response code
	if code, ok := result["code"].(float64); ok && code != 0 {
		msg := "unknown error"
		if m, ok := result["msg"].(string); ok {
			msg = m
		}
		return "", 0, fmt.Errorf("login failed with code %.0f: %s", code, msg)
	}

	// Extract token and expiresAt from response
	// Response format: {"code": 0, "data": {"token": "...", "expiresAt": 1234567890}}
	var token string
	var expiresAt int64

	if data, ok := result["data"].(map[string]interface{}); ok {
		// Get token
		if tkn, ok := data["token"].(string); ok {
			token = tkn
		}

		// Get expiresAt
		if exp, ok := data["expiresAt"].(float64); ok {
			expiresAt = int64(exp)
		}
	}

	if token == "" {
		return "", 0, fmt.Errorf("token not found in login response")
	}

	return token, expiresAt, nil
}

// retryableTransport implements HTTP retries
type retryableTransport struct {
	Transport  http.RoundTripper
	MaxRetries int
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i <= t.MaxRetries; i++ {
		if i > 0 {
			log.Debug("Retrying HTTP request", "attempt", i, "url", req.URL)
			time.Sleep(time.Duration(i) * time.Second)
		}

		resp, err = t.Transport.RoundTrip(req)
		if err == nil {
			// Check for retryable status codes
			if resp.StatusCode < 500 && resp.StatusCode != 429 {
				return resp, nil
			}
			resp.Body.Close()
		}
	}

	return resp, err
}
