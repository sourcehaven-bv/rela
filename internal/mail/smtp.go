package mail

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// SMTPSender delivers over authenticated SMTP with mandatory STARTTLS.
type SMTPSender struct {
	cfg     Config
	rootCAs *x509.CertPool
}

// SMTPOption adjusts an SMTPSender.
//
// Go-only, with no YAML counterpart, and deliberately so: the one thing an
// operator might reach for here is "skip certificate verification", which would
// hollow out the STARTTLS guarantee in exactly the deployments that need it.
// Tests supply a trusted CA instead, which exercises the real verification path.
type SMTPOption func(*SMTPSender)

// WithRootCAs pins the certificate pool used to verify the server.
//
// Intended for tests against a local server with a self-signed certificate.
// Verification still happens — this changes WHO is trusted, not WHETHER trust
// is checked.
func WithRootCAs(pool *x509.CertPool) SMTPOption {
	return func(s *SMTPSender) { s.rootCAs = pool }
}

// NewSMTPSender returns a sender for cfg.
//
// Nil: a nil config is rejected. The config is also re-validated here rather
// than trusted from load, so a programmatically constructed Config cannot
// bypass the checks that the YAML path enforces.
func NewSMTPSender(cfg *Config, opts ...SMTPOption) (*SMTPSender, error) {
	if cfg == nil {
		return nil, errors.New("mail: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Transport != TransportSMTP {
		return nil, fmt.Errorf("mail: NewSMTPSender given transport %q", cfg.Transport)
	}
	s := &SMTPSender{cfg: *cfg}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Send delivers m.
//
// TLS policy is set EXPLICITLY to mandatory rather than left to the library
// default. go-mail's default happens to be mandatory today, but a dependency
// bump could change it, and the opportunistic policy silently falls back to
// unencrypted port 25 — which would turn "STARTTLS required" into a comment
// rather than a control. Certificate verification is left on; there is no
// skip-verify knob in the config, deliberately, because one would make the
// guarantee meaningless in exactly the deployments that most need it.
func (s *SMTPSender) Send(ctx context.Context, m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}

	msg, err := s.buildMessage(m)
	if err != nil {
		return err
	}

	password := s.cfg.resolvePassword()
	if s.cfg.Username != "" && password == "" {
		// Fail fast rather than authenticating with an empty password: the
		// server rejects it, the outbox retries five times with backoff, and
		// the relay may lock the account out — 30s of noise and a possible
		// lockout for what is a one-line configuration mistake.
		return fmt.Errorf("mail: username %q is set but %s is empty or unset",
			s.cfg.Username, s.cfg.PasswordEnv)
	}

	opts := []gomail.Option{
		gomail.WithPort(s.cfg.EffectivePort()),
		gomail.WithTLSPolicy(gomail.TLSMandatory),
		gomail.WithTimeout(time.Duration(s.cfg.Timeout()) * time.Second),
	}
	if s.rootCAs != nil {
		opts = append(opts, gomail.WithTLSConfig(&tls.Config{
			RootCAs:    s.rootCAs,
			ServerName: s.cfg.Host,
			MinVersion: tls.VersionTLS12,
		}))
	}
	if s.cfg.Username != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(s.cfg.Username),
			gomail.WithPassword(password),
		)
	}

	client, err := gomail.NewClient(s.cfg.Host, opts...)
	if err != nil {
		return s.wrap("build smtp client", err, password)
	}

	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return s.wrap("send", err, password)
	}
	return nil
}

// wrap annotates an error, scrubbing the credential.
//
// An SMTP server's rejection can echo parts of the exchange, and these errors
// travel into logs and, via `rela validate`, onto a terminal. The structure of
// the code already keeps the password out of most paths; this is the backstop
// for the ones it does not.
func (s *SMTPSender) wrap(op string, err error, password string) error {
	return fmt.Errorf("mail: %s: %s", op, redact(err.Error(), password))
}

// buildMessage converts a Message into a go-mail message.
//
// Body parts are set from already-rendered bytes; nothing is rendered here. The
// HTML part is set as the primary content type with the text part as its
// alternative, which is the multipart/alternative ordering mail clients expect
// (least-capable first on the wire, which go-mail handles).
func (s *SMTPSender) buildMessage(m Message) (*gomail.Msg, error) {
	msg := gomail.NewMsg()

	if s.cfg.FromName != "" {
		if err := msg.FromFormat(s.cfg.FromName, s.cfg.From); err != nil {
			return nil, fmt.Errorf("mail: from: %w", err)
		}
	} else if err := msg.From(s.cfg.From); err != nil {
		return nil, fmt.Errorf("mail: from: %w", err)
	}

	for _, a := range m.To {
		var err error
		if a.Name != "" {
			err = msg.AddToFormat(a.Name, a.Email)
		} else {
			err = msg.AddTo(a.Email)
		}
		if err != nil {
			return nil, fmt.Errorf("mail: recipient %q: %w", a.Email, err)
		}
	}

	msg.Subject(m.Subject)

	switch {
	case len(m.HTML) > 0 && len(m.Text) > 0:
		msg.SetBodyString(gomail.TypeTextHTML, string(m.HTML))
		msg.AddAlternativeString(gomail.TypeTextPlain, string(m.Text))
	case len(m.HTML) > 0:
		msg.SetBodyString(gomail.TypeTextHTML, string(m.HTML))
	default:
		msg.SetBodyString(gomail.TypeTextPlain, string(m.Text))
	}

	for _, img := range m.InlineImages {
		data := img.Data
		if err := msg.EmbedReader(img.CID, newByteReader(data),
			gomail.WithFileContentType(gomail.ContentType(img.ContentType))); err != nil {
			return nil, fmt.Errorf("mail: embed %q: %w", img.CID, err)
		}
	}

	return msg, nil
}
