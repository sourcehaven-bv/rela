// Package git provides git operations for the data entry app.
package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceCodeResponse is returned when initiating the device flow.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is returned when the device is authorized.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// TokenErrorResponse is returned when polling for token fails.
type TokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Common OAuth errors.
var (
	ErrAuthorizationPending = errors.New("authorization pending")
	ErrSlowDown             = errors.New("slow down")
	ErrExpiredToken         = errors.New("device code expired")
	ErrAccessDenied         = errors.New("access denied by user")
)

// OAuthConfig holds GitHub OAuth configuration.
type OAuthConfig struct {
	ClientID string // GitHub OAuth App client ID
}

// OAuth provides GitHub OAuth device flow authentication.
type OAuth struct {
	config OAuthConfig
	client *http.Client
}

// NewOAuth creates a new OAuth instance.
func NewOAuth(cfg OAuthConfig) *OAuth {
	return &OAuth{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// RequestDeviceCode initiates the device flow and returns a code for the user.
func (o *OAuth) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", o.config.ClientID)
	data.Set("scope", "repo")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/device/code",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %s", string(body))
	}

	var result DeviceCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// PollForToken polls GitHub until the user authorizes the device or the code expires.
// Returns ErrAuthorizationPending if still waiting, ErrExpiredToken if expired.
func (o *OAuth) PollForToken(ctx context.Context, deviceCode string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", o.config.ClientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll for token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check for error response first
	var errResp TokenErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		switch errResp.Error {
		case "authorization_pending":
			return nil, ErrAuthorizationPending
		case "slow_down":
			return nil, ErrSlowDown
		case "expired_token":
			return nil, ErrExpiredToken
		case "access_denied":
			return nil, ErrAccessDenied
		default:
			return nil, fmt.Errorf("%s: %s", errResp.Error, errResp.ErrorDescription)
		}
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, errors.New("no access token in response")
	}

	return &result, nil
}

// minPollInterval is the minimum polling interval in seconds.
const minPollInterval = 5

// slowDownIncrement is added to the interval when GitHub returns slow_down.
const slowDownIncrement = 5

// WaitForAuthorization polls until authorized or context cancelled.
// It respects the interval from the device code response.
func (o *OAuth) WaitForAuthorization(ctx context.Context, deviceCode string, interval int) (*TokenResponse, error) {
	if interval < minPollInterval {
		interval = minPollInterval
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := o.PollForToken(ctx, deviceCode)
			if err == nil {
				return token, nil
			}
			if errors.Is(err, ErrAuthorizationPending) {
				continue
			}
			if errors.Is(err, ErrSlowDown) {
				// Increase interval
				ticker.Reset(time.Duration(interval+slowDownIncrement) * time.Second)
				continue
			}
			return nil, err
		}
	}
}
