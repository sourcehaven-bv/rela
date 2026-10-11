package tokenstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxTokenResponse caps the token endpoint's response body.
const maxTokenResponse = 1 << 20

// errInvalidGrant marks a refresh token the provider refused for good.
var errInvalidGrant = errors.New("invalid_grant")

// oauthErrorCode matches an RFC 6749 error code. Only a value of this shape
// is repeated in an error message; anything else from the body is not.
var oauthErrorCode = regexp.MustCompile(`^[a-z_]{1,64}$`)

// tokenResponse is the RFC 6749 section 5.1 response. expires_in is a number
// in the RFC, but some providers send a numeric string.
type tokenResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    json.RawMessage `json:"expires_in"`
	Error        string          `json:"error"`
}

// refresh exchanges old's refresh token for a new access token.
//
// The request runs on a context the caller cannot cancel, bounded by
// refreshTimeout: once the provider has the request it may rotate the
// refresh token, and abandoning the response would lose the only valid one.
//
// No error carries the token URL, the request or response body, or a
// secret. Errors reach script output, logs and job records.
func (b *Broker) refresh(ctx context.Context, c Connection, old Token) (Token, error) {
	secrets, err := b.secrets.Secrets("")
	if err != nil {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: reading secrets failed", c.Name)
	}
	clientID, clientSecret := secrets[c.ClientIDSecret], secrets[c.ClientSecretSecret]
	if clientID == "" || clientSecret == "" {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: secrets %q and %q must both be set",
			c.Name, c.ClientIDSecret, c.ClientSecretSecret)
	}

	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old.Refresh}}
	if c.Style == StyleLaunchpad {
		// Launchpad (37signals) reads its older `type=refresh` as well as
		// grant_type; sending both works with either.
		form.Set("type", "refresh")
		form.Set("client_id", clientID)
		form.Set("client_secret", clientSecret)
	}

	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: token_url is not a valid request URL", c.Name)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.Style == StyleRFC6749 {
		// RFC 6749 section 2.3.1: form-encode both parts before Basic.
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(clientSecret))
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: %s", c.Name, transportReason(err))
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponse))
	if err != nil {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: reading the response failed", c.Name)
	}

	var tr tokenResponse
	jsonErr := json.Unmarshal(body, &tr)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized) &&
			jsonErr == nil && tr.Error == "invalid_grant" {

			return Token{}, errInvalidGrant
		}
		code := ""
		if jsonErr == nil && oauthErrorCode.MatchString(tr.Error) {
			code = " (" + tr.Error + ")"
		}
		return Token{}, fmt.Errorf("tokenstore: refresh %q: token endpoint answered %d%s",
			c.Name, resp.StatusCode, code)
	}
	if jsonErr != nil || tr.AccessToken == "" {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: token endpoint answered %d without an access token",
			c.Name, resp.StatusCode)
	}

	lifetime, err := parseExpiresIn(tr.ExpiresIn)
	if err != nil {
		return Token{}, fmt.Errorf("tokenstore: refresh %q: %w", c.Name, err)
	}
	next := Token{Refresh: old.Refresh, Access: tr.AccessToken, ExpiresAt: b.now().Add(lifetime)}
	if tr.RefreshToken != "" {
		next.Refresh = tr.RefreshToken
	}
	return next, nil
}

// parseExpiresIn reads expires_in as a number or a numeric string. Absent,
// null or 0 means the provider named no lifetime, and an hour is assumed.
// NaN and infinity are refused; a lifetime over a year is read as a year.
func parseExpiresIn(raw json.RawMessage) (time.Duration, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return defaultExpiry, nil
	}
	if unq, err := strconv.Unquote(s); err == nil {
		s = strings.TrimSpace(unq)
	}
	secs, err := strconv.ParseFloat(s, 64)
	if err != nil || secs < 0 || math.IsNaN(secs) || math.IsInf(secs, 0) {
		return 0, errors.New("expires_in is not a number of seconds")
	}
	if secs == 0 {
		return defaultExpiry, nil
	}
	if secs >= maxExpiry.Seconds() {
		return maxExpiry, nil
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// transportReason describes a failed request without the URL that
// *url.Error would print.
func transportReason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "the token endpoint did not answer in time"
	case errors.Is(err, context.Canceled):
		return "the request was canceled"
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		if uerr.Timeout() {
			return "the token endpoint did not answer in time"
		}
		return "request failed: " + uerr.Err.Error()
	}
	return "request failed"
}
