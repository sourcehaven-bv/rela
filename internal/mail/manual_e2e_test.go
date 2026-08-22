//go:build mailmanual

package mail_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// TestManual_EndToEnd renders a realistic digest and delivers it through the
// outbox over real SMTP+STARTTLS, then writes the received message to disk for
// inspection.
//
// Build-tag gated (`-tags mailmanual`) rather than "temporary code deleted
// before merge": a manual harness that has to be remembered is how debug code
// reaches production. CI never builds this file.
//
//	go test -tags mailmanual ./internal/mail -run TestManual_EndToEnd -v
func TestManual_EndToEnd(t *testing.T) {
	fake, pool := newFakeSMTP(t, true)
	t.Setenv("RELA_MANUAL_SMTP_PASSWORD", "manual-secret")

	// 1. Render, exactly as a caller would.
	r, err := mailrender.New(&mailrender.Options{
		BaseURL: "https://rela.example.com",
		LogoCID: "logo@rela",
		Palette: map[string]string{"--accent-color": "#2b6cb0"},
	})
	require.NoError(t, err)

	html, text, err := r.Render(&mailrender.Message{
		Subject: "3 tasks need you — Tâches en retard",
		Intro:   "Good morning. **3 items** need attention. See [the board](/board).",
		Sections: []mailrender.Section{
			{
				Title:   "Overdue",
				Body:    "These are _past_ their due date.",
				Columns: []string{"Title", "Due", "Owner"},
				Rows: [][]string{
					{"Ship the mail feature", "2026-08-01", "jeroen"},
					{"Review <script>alert(1)</script>", "2026-08-14", "alex"},
				},
				Links: []string{"/e/TKT-332QZY", "/e/TKT-U2R7GU"},
			},
			{Title: "Nothing due", Columns: []string{"Title"}},
		},
		Footer: "Sent by rela.",
	})
	require.NoError(t, err)

	// 2. Send through the outbox, as production does.
	sender, err := mail.NewSMTPSender(&mail.Config{
		Transport:   mail.TransportSMTP,
		Host:        fake.host(),
		Port:        fake.port(),
		Username:    "relay",
		PasswordVar: "RELA_MANUAL_SMTP_PASSWORD",
		From:        "rela@example.com",
		FromName:    "Rela",
	}, mail.WithRootCAs(pool))
	require.NoError(t, err)

	ob, err := mail.NewOutbox(sender, mail.OutboxConfig{})
	require.NoError(t, err)
	ob.Start()
	defer ob.Stop()

	pngPixel := []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	}
	require.NoError(t, ob.Enqueue(mail.Message{
		To:          []mail.Address{{Email: "jeroen@example.com", Name: "Jeroen"}},
		Subject:     "3 tasks need you — Tâches en retard",
		HTML:        html,
		Text:        text,
		RenderedFor: "user:jeroen",
		InlineImages: []mail.InlineImage{
			{CID: "logo@rela", ContentType: "image/png", Data: pngPixel},
		},
	}))

	require.Eventually(t, func() bool { return len(fake.received()) == 1 },
		10*time.Second, 20*time.Millisecond, "message never arrived")

	got := fake.received()[0]

	dir := "/tmp/mailmanual"
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(dir+"/received.eml", []byte(got.Data), 0o644))
	require.NoError(t, os.WriteFile(dir+"/rendered.html", html, 0o644))
	require.NoError(t, os.WriteFile(dir+"/rendered.txt", text, 0o644))

	user, pass := fake.credentials()
	fmt.Printf("\n=== MANUAL VERIFICATION ===\nfrom=%s to=%v auth=%s/%s bytes=%d\nwrote %s/received.eml\n",
		got.From, got.To, user, pass, len(got.Data), dir)

	_ = context.Background()
}
