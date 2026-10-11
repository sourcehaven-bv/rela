package tokenstore_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

func TestParseInput(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name, in string
		want     tokenstore.Token
		wantErr  string
	}{
		{name: "bare token", in: "  abc.def\n", want: tokenstore.Token{Refresh: "abc.def"}},
		{
			name: "json with access", in: `{"refresh_token":"r","access_token":"a","expires_in":"7200"}`,
			want: tokenstore.Token{Refresh: "r", Access: "a", ExpiresAt: now.Add(2 * time.Hour)},
		},
		{
			name: "json without expiry", in: `{"refresh_token":"r","access_token":"a"}`,
			want: tokenstore.Token{Refresh: "r", Access: "a", ExpiresAt: now.Add(time.Hour)},
		},
		{name: "json refresh only", in: `{"refresh_token":"r"}`, want: tokenstore.Token{Refresh: "r"}},
		{name: "empty", in: " \n", wantErr: "no token"},
		{name: "two words", in: "a b", wantErr: "one line"},
		{name: "bad json", in: "{nope", wantErr: "not a JSON object"},
		{name: "json without refresh", in: `{"access_token":"a"}`, wantErr: "no refresh_token"},
		{name: "bad expiry", in: `{"refresh_token":"r","access_token":"a","expires_in":"soon"}`, wantErr: "expires_in"},
		{name: "too long", in: strings.Repeat("x", 17<<10), wantErr: "too long"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tokenstore.ParseInput([]byte(tc.in), now)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
