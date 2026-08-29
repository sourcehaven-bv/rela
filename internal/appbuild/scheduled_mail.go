package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdmail "net/mail"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
	"github.com/Sourcehaven-BV/rela/internal/mailtemplate"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunScheduledTemplate renders and sends one message under the principal
// already installed by scheduler's recipient child handler.
func (s *Services) RunScheduledTemplate(ctx context.Context, name, recipientID string) error {
	if s.mail == nil || s.mail.sender == nil || s.mail.config == nil {
		return errors.New("mail is not configured")
	}
	data, err := s.cfgLoader.Load(ctx, mailtemplate.ConfigFile)
	if err != nil {
		return fmt.Errorf("load %s: %w", mailtemplate.ConfigFile, err)
	}
	cfg, err := mailtemplate.Parse(data, s.meta)
	if err != nil {
		return err
	}
	tmpl, ok := cfg.Templates[name]
	if !ok {
		return fmt.Errorf("unknown mail template %q", name)
	}

	// The raw recipient record is used only to address the envelope. It is
	// never supplied to content rendering, which remains ACL-visible.
	recipient, err := s.store.GetEntity(ctx, recipientID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	raw, ok := recipient.Properties[tmpl.AddressProperty]
	if !ok {
		return skipBadAddress(ctx, recipientID, tmpl.AddressProperty)
	}
	addressText, ok := raw.(string)
	if !ok || strings.TrimSpace(addressText) == "" {
		return skipBadAddress(ctx, recipientID, tmpl.AddressProperty)
	}
	parsed, err := stdmail.ParseAddress(addressText)
	if err != nil {
		return skipBadAddress(ctx, recipientID, tmpl.AddressProperty)
	}

	deps := s.ScheduledLuaWriteDeps()
	model, err := mailtemplate.Build(ctx, s.meta, deps.VisibleReader, tmpl, time.Now())
	if err != nil {
		return err
	}
	renderer, err := mailrender.New(&mailrender.Options{BaseURL: s.mail.config.BaseURL})
	if err != nil {
		return err
	}
	html, text, err := renderer.Render(model)
	if err != nil {
		return err
	}
	return s.mail.sender.Send(ctx, mail.Message{
		To:      []mail.Address{{Email: parsed.Address, Name: parsed.Name}},
		Subject: model.Subject, HTML: html, Text: text, RenderedFor: recipientID,
	})
}

func skipBadAddress(ctx context.Context, recipientID, property string) error {
	slog.WarnContext(ctx, "scheduled mail recipient has no usable address; skipping",
		"recipient", recipientID, "property", property)
	return nil
}
