package lua

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// fakeHost is a HostConfig over fixed files and secrets.
type fakeHost struct {
	files   map[string]string
	secrets map[string]string
}

func (h fakeHost) File(name string) ([]byte, error) {
	if v, ok := h.files[name]; ok {
		return []byte(v), nil
	}
	return nil, fs.ErrNotExist
}

func (h fakeHost) Secrets(string) (map[string]string, error) {
	if h.secrets == nil {
		return nil, secrets.ErrNotFound
	}
	return h.secrets, nil
}

func (fakeHost) Path() string { return "" }

func TestLoadContextOptions_Host(t *testing.T) {
	tests := []struct {
		name     string
		host     HostConfig
		script   string
		mail     MailSenderLoader
		wantOpts int
		wantErr  bool
	}{
		{name: "nil host is refused", wantErr: true},
		{name: "nothing configured", host: fakeHost{}, script: "s.lua"},
		{name: "ai and secrets", script: "s.lua", wantOpts: 2, host: fakeHost{
			files:   map[string]string{"ai.yaml": "base_url: http://localhost:1/v1\nmodel: m\n"},
			secrets: map[string]string{"ai_api_key": "k"},
		}},
		{name: "inline code gets no secrets", wantOpts: 0, host: fakeHost{secrets: map[string]string{"a": "b"}}},
		{name: "broken ai.yaml is an error", wantErr: true, host: fakeHost{files: map[string]string{"ai.yaml": "model: m\n"}}},
		{name: "broken mail is an error", wantErr: true, host: fakeHost{},
			mail: func(HostConfig) (MailSender, error) { return nil, errors.New("bad mail.yaml") }},
		{name: "mail loader gets the host", host: fakeHost{},
			mail: func(h HostConfig) (MailSender, error) {
				if h == nil {
					return nil, errors.New("no host")
				}
				return nil, nil //nolint:nilnil // MailSenderLoader: (nil, nil) means mail is not configured
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := LoadContextOptions(tc.host, tc.script, tc.mail)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(opts) != tc.wantOpts {
				t.Errorf("got %d options, want %d", len(opts), tc.wantOpts)
			}
		})
	}
}

// withheldHost withholds ai.yaml and the secrets, as tenant mode does.
type withheldHost struct{}

var errTestWithheld = fmt.Errorf("withheld in this test: %w", ErrHostConfigWithheld)

func (withheldHost) File(string) ([]byte, error)               { return nil, errTestWithheld }
func (withheldHost) Secrets(string) (map[string]string, error) { return nil, errTestWithheld }
func (withheldHost) Path() string                              { return "" }

// TestLoadContextOptions_Withheld pins that a withheld ai.yaml and withheld
// secrets do not stop the runtime, and that a script using them gets an
// error naming why rather than empty values.
func TestLoadContextOptions_Withheld(t *testing.T) {
	opts, err := LoadContextOptions(withheldHost{}, "s.lua", nil)
	if err != nil {
		t.Fatalf("LoadContextOptions: %v", err)
	}
	opts = append(opts, WithCapabilities(Capabilities{AI: true, AllSecrets: true}))
	rt := NewDetached(io.Discard, opts...)
	defer rt.Close()

	err = rt.RunString(`local r, e = ai.complete("hi")
assert(r == nil and e.kind == "not_configured", "ai.complete did not refuse")
error("ai: " .. e.message)`)
	if err == nil || !strings.Contains(err.Error(), "ai: AI is not available: withheld in this test") {
		t.Errorf("ai.complete error = %v; want not_configured naming why", err)
	}

	err = rt.RunString(`return rela.secrets.token`)
	if err == nil || !strings.Contains(err.Error(), "withheld in this test") {
		t.Fatalf("reading a withheld secret = %v; want an error naming why", err)
	}
}
