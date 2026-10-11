package tokenstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode"
)

// maxInput bounds what ParseInput accepts. A refresh token is a few hundred
// bytes; the bound only keeps a mistaken pipe from being read whole.
const maxInput = 16 << 10

// ParseInput reads what an operator hands `rela token set` or the desktop
// Connections section: either a bare refresh token, or the provider's JSON
// token response (`refresh_token`, optional `access_token` and
// `expires_in`). Pasting the JSON keeps the access token the consent flow
// already obtained, so the first sync needs no refresh.
func ParseInput(data []byte, now time.Time) (Token, error) {
	if len(data) > maxInput {
		return Token{}, errors.New("input is too long for a token")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Token{}, errors.New("no token given")
	}
	if data[0] != '{' {
		if bytes.ContainsFunc(data, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
			return Token{}, errors.New("a refresh token is one line without spaces; " +
				"paste the token, or the provider's JSON token response")
		}
		return Token{Refresh: string(data)}, nil
	}
	var body struct {
		RefreshToken string          `json:"refresh_token"`
		AccessToken  string          `json:"access_token"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return Token{}, errors.New("input starts with '{' but is not a JSON object")
	}
	if body.RefreshToken == "" {
		return Token{}, errors.New("the JSON has no refresh_token")
	}
	t := Token{Refresh: body.RefreshToken}
	if body.AccessToken != "" {
		ttl, err := parseExpiresIn(body.ExpiresIn)
		if err != nil {
			return Token{}, fmt.Errorf("the JSON token response: %w", err)
		}
		t.Access, t.ExpiresAt = body.AccessToken, now.Add(ttl).UTC()
	}
	return t, nil
}
