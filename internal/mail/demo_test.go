//go:build maildemo

package mail_test

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// TestDemo_Mailpit drives the real send path against Mailpit, a third-party
// SMTP server, with STARTTLS required and certificate verification on.
//
// Build-tag gated (`-tags maildemo`) so CI never runs it. Start Mailpit first:
//
//	mailpit --smtp 127.0.0.1:1025 --smtp-tls-cert cert.pem --smtp-tls-key key.pem \
//	        --smtp-require-starttls --smtp-auth-accept-any
func TestDemo_Mailpit(t *testing.T) {
	certPEM, err := os.ReadFile("/tmp/maildemo/cert.pem")
	require.NoError(t, err, "mailpit cert not found — is the demo server running?")
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certPEM), "bad cert")

	// Credential from .rela/secrets.yaml, the way an operator would set it.
	relaDir := t.TempDir()
	require.NoError(t, os.WriteFile(relaDir+"/secrets.yaml",
		[]byte("smtp_password: demo-password-do-not-log\n"), 0o600))

	// 1. Render a realistic digest, including hostile content in a cell.
	r, err := mailrender.New(&mailrender.Options{
		BaseURL: "https://rela.example.com",
		Palette: map[string]string{"--accent-color": "#2b6cb0"},
	})
	require.NoError(t, err)

	html, text, err := r.Render(&mailrender.Message{
		Subject: "3 tasks need you — Tâches en retard",
		Intro: "Good morning. **3 items** need attention. " +
			"See [the board](/board) or [the docs](https://docs.example.com/guide_(v2)).",
		Sections: []mailrender.Section{
			{
				Title:   "Overdue",
				Body:    "These are _past_ their due date.",
				Columns: []string{"Title", "Due", "Owner"},
				Rows: [][]string{
					{"Ship the mail feature", "2026-08-01", "jeroen"},
					{"Review &lt;script&gt;alert(1)&lt;/script&gt;", "2026-08-14", "alex"},
					{"Cost is 5*3*2 dollars", "2026-08-20", "sam"},
				},
				Links: []string{"/e/TKT-332QZY", "/e/TKT-U2R7GU", ""},
			},
			{
				Title: "Notes",
				Body:  "## Agenda\n\n- one\n- two\n\n| col a | col b |\n|---|---|\n| 1 | 2 |",
			},
			{Title: "Nothing due", Columns: []string{"Title"}},
		},
		Footer: "Sent by rela.",
	})
	require.NoError(t, err)

	// 2. Send it through the outbox over real SMTP + STARTTLS.
	sender, err := mail.NewSMTPSender((&mail.Config{
		Transport: mail.TransportSMTP,
		Host:      "127.0.0.1",
		Port:      1025,
		Username:  "demo",
		From:      "rela@example.com",
		FromName:  "Rela",
		BaseURL:   "https://rela.example.com",
	}).WithRelaDir(relaDir), mail.WithRootCAs(pool))
	require.NoError(t, err)

	ob, err := mail.NewOutbox(sender, mail.OutboxConfig{})
	require.NoError(t, err)
	ob.Start()
	defer ob.Stop()

	require.NoError(t, ob.Enqueue(mail.Message{
		To: []mail.Address{
			{Email: "jeroen@example.com", Name: "Jeroen"},
			{Email: "alex@example.com"},
		},
		Subject:     "3 tasks need you — Tâches en retard",
		HTML:        html,
		Text:        text,
		RenderedFor: "user:jeroen",
	}))

	fmt.Println("\n=== enqueued; waiting for delivery ===")
	require.Eventually(t, func() bool {
		return mailpitCount(t) >= 1
	}, 20*time.Second, 200*time.Millisecond, "message never reached Mailpit")

	fmt.Println("=== delivered. Open http://127.0.0.1:8025 to view it ===")
	_ = context.Background()
}

// mailpitCount returns how many messages Mailpit currently holds.
func mailpitCount(t *testing.T) int {
	t.Helper()
	resp, err := httpGet("http://127.0.0.1:8025/api/v1/messages?limit=1")
	if err != nil {
		return 0
	}
	var out struct {
		Total int `json:"total"`
	}
	if json.Unmarshal(resp, &out) != nil {
		return 0
	}
	return out.Total
}

// demoClient disables keep-alives so goleak does not trip over idle
// connections this demo's polling would otherwise leave behind. The mail path
// itself owns no HTTP client; this is purely the test talking to Mailpit's API.
var demoClient = &http.Client{
	Timeout:   5 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

func httpGet(url string) ([]byte, error) {
	resp, err := demoClient.Get(url) //nolint:gosec,noctx // demo-only, fixed localhost URL
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
