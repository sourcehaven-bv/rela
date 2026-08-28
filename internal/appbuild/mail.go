package appbuild

import (
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// startMail loads .rela/mail.yaml and, if mail is configured, builds a sender
// and starts its outbox worker for one assembled store.
//
// Returns the outbox (nil when mail is off) and a stop function that is always
// safe to call. Following startDataMigration: every "not configured" and
// "failed to start" branch returns a no-op stop rather than an error, because
// mail must never fail boot — a project with no mail.yaml, or with a mail
// server that is currently unreachable, still has to serve every other command.
//
// Note what does NOT happen here: a nil sender is never substituted for a real
// one. "Mail is off" is represented by a nil *mail.Outbox that callers check,
// not by a Sender that silently discards messages — the latter would turn a
// wiring mistake into mail that vanishes without trace.
//
// The worker is per-assembled, exactly like the GC sweep: Assemble runs once
// per store, so a multi-tenant deployment gets one worker (and one SMTP
// connection) per tenant. That satisfies the SharedBase rule that Close tears
// down only per-assembled resources, at the cost of N connections to a shared
// provider — documented in docs/mail.md rather than hidden here.
func startMail(paths *project.Context) (ob *mail.Outbox, stop func()) {
	noop := func() {}

	if paths == nil {
		return nil, noop
	}

	cfg, err := mail.LoadConfig(paths.CacheDir)
	switch {
	case errors.Is(err, mail.ErrConfigNotFound):
		// Mail is not configured. The overwhelmingly common case, and not
		// worth a log line on every command.
		return nil, noop
	case err != nil:
		// Configured but broken: say so loudly. A silent skip here would look
		// identical to "not configured" and leave an operator debugging a
		// typo with no signal at all.
		slog.Warn("mail: disabled — invalid .rela/mail.yaml", "error", err)
		return nil, noop
	}

	sender, err := senderFor(cfg)
	if err != nil {
		slog.Warn("mail: disabled — cannot build sender", "error", err)
		return nil, noop
	}

	outbox, err := mail.NewOutbox(sender, mail.OutboxConfig{})
	if err != nil {
		slog.Warn("mail: disabled — cannot build outbox", "error", err)
		return nil, noop
	}
	outbox.Start()

	slog.Info("mail: enabled", "transport", cfg.Transport)
	return outbox, outbox.Stop
}

// senderFor builds the transport named by the config.
//
// The switch is exhaustive over a closed set; an unknown transport cannot reach
// here because Validate rejects it at load, but the default arm stays as a
// compile-time reminder that a new transport needs wiring in both places.
func senderFor(cfg *mail.Config) (mail.Sender, error) {
	switch cfg.Transport {
	case mail.TransportSMTP:
		return mail.NewSMTPSender(cfg)
	case mail.TransportMemory:
		return mail.NewMemorySender(0), nil
	default:
		return nil, errors.New("mail: unsupported transport " + string(cfg.Transport))
	}
}
