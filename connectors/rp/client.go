package rp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client is a high-level client for the Report Portal API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// Option configures the Client during construction.
type Option func(*clientConfig) error

type clientConfig struct {
	httpClient *http.Client
	logger     *slog.Logger
	timeout    time.Duration
}

// New creates a new Client for the given Report Portal instance.
// The bearerToken is sent as an Authorization header on every request.
func New(baseURL, bearerToken string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("rp: baseURL is required")
	}
	if !strings.HasPrefix(baseURL, "https://") &&
		!strings.HasPrefix(baseURL, "http://localhost") &&
		!strings.HasPrefix(baseURL, "http://127.0.0.1") {
		return nil, fmt.Errorf("rp: baseURL must use HTTPS (got %q)", baseURL)
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	cfg := &clientConfig{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if cfg.timeout > 0 {
		httpClient.Timeout = cfg.timeout
	}

	logger := cfg.logger
	if logger == nil {
		logger = slog.Default().With("component", "rp-client")
	}

	return &Client{
		baseURL:    baseURL,
		token:      bearerToken,
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *clientConfig) error {
		cfg.httpClient = c
		return nil
	}
}

// WithLogger configures structured logging.
func WithLogger(l *slog.Logger) Option {
	return func(cfg *clientConfig) error {
		cfg.logger = l
		return nil
	}
}

// WithTimeout sets a timeout on the HTTP client.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) error {
		cfg.timeout = d
		return nil
	}
}

// doJSON executes an HTTP request and decodes the JSON response into dst.
// If the response has an error status, it returns an *APIError.
func (c *Client) doJSON(ctx context.Context, method, url, operation string, body io.Reader, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("%s: create request: %w", operation, err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.InfoContext(ctx, "API request", "operation", operation, "method", method, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: do request: %w", operation, err)
	}
	defer resp.Body.Close()

	c.logger.DebugContext(ctx, "API response", "operation", operation, "status", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		var errRS ErrorRS
		if json.Unmarshal(respBody, &errRS) == nil && errRS.Message != "" {
			return newAPIError(operation, resp.StatusCode, errRS.ErrorCode, errRS.Message)
		}
		msg := string(respBody)
		if msg == "" {
			msg = resp.Status
		}
		return newAPIError(operation, resp.StatusCode, 0, msg)
	}

	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("%s: decode response: %w", operation, err)
		}
	}
	return nil
}

// GetCurrentUser returns the authenticated user's profile based on the bearer token.
// Uses GET /users which resolves the user from the token.
func (c *Client) GetCurrentUser(ctx context.Context) (*UserResource, error) {
	u := fmt.Sprintf("%s/users", c.baseURL)
	var user UserResource
	if err := c.doJSON(ctx, "GET", u, "get current user", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ReadAPIKey reads the first line of a file (e.g. .rp-api-key) and returns it trimmed.
func ReadAPIKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.Split(string(data), "\n")[0])
	return line, nil
}
