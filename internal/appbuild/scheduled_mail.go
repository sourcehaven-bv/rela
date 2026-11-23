package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdmail "net/mail"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mailbrand"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
	"github.com/Sourcehaven-BV/rela/internal/mailtemplate"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunScheduledTemplate renders and sends one message under the principal
// already installed by scheduler's recipient child handler.
func (w *SchedulerWorkspace) RunScheduledTemplate(ctx context.Context, name, recipientID string) error {
	if w.mail == nil || w.mail.sender == nil || w.mail.config == nil {
		return errors.New("mail is not configured")
	}
	data, err := w.cfgLoader.Load(ctx, mailtemplate.ConfigFile)
	if err != nil {
		return fmt.Errorf("load %s: %w", mailtemplate.ConfigFile, err)
	}
	cfg, err := mailtemplate.Parse(data, w.meta)
	if err != nil {
		return err
	}
	tmpl, ok := cfg.Templates[name]
	if !ok {
		return fmt.Errorf("unknown mail template %q", name)
	}

	// The raw recipient record is used only to address the envelope. It is
	// never supplied to content rendering, which remains ACL-visible. It is
	// the face the default world selects, the world scheduled mail iterates
	// in (TKT-KQXVF7 D7).
	recipient, found, err := w.defaultWorldRow(ctx, recipientID)
	if err != nil {
		return err
	}
	if !found {
		slog.WarnContext(ctx, "scheduled mail recipient has no row in the default world; skipping",
			"recipient", recipientID)
		return nil
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

	deps := w.ScheduledLuaWriteDeps()
	world := w.worlds.DefaultWorld()
	model, contributed, err := mailtemplate.Build(ctx, w.meta, deps.VisibleReader, world, tmpl, w.clock())
	if err != nil {
		return err
	}
	if tmpl.RequireVisibleContent && contributed == 0 {
		return skipEmptyContent(ctx, name, recipientID)
	}
	opts := scheduledMailOptions(ctx, w)
	renderer, err := mailrender.New(&opts)
	if err != nil {
		return err
	}
	html, text, err := renderer.Render(model)
	if err != nil {
		return err
	}
	return w.mail.sender.Send(ctx, mail.Message{
		To:      []mail.Address{{Email: parsed.Address, Name: parsed.Name}},
		Subject: model.Subject, HTML: html, Text: text, RenderedFor: recipientID,
	})
}

// scheduledMailOptions returns the renderer options for a scheduled mail: the
// mail.yaml base URL with the app's accent color and logo. A branding read
// error is logged and the defaults fill in, so a bad palette file does not stop
// the mail. A function rather than a method: it is only ever called from
// [SchedulerWorkspace.RunScheduledTemplate].
func scheduledMailOptions(ctx context.Context, w *SchedulerWorkspace) mailrender.Options {
	opts, err := mailbrand.Source{Config: w.cfgLoader, KV: w.stateKV}.RenderOptions(ctx, w.mail.config.BaseURL)
	if err != nil {
		slog.WarnContext(ctx, "scheduled mail: app branding not fully read; using defaults for the rest", "error", err)
	}
	return opts
}

// servicesNow reads the clock of the base s was assembled from ([WithClock]),
// or the wall clock for a bundle built without one. A function rather than a
// method: Services is at its plimsoll method line.
func servicesNow(s *Services) time.Time {
	if s.base == nil {
		return time.Now()
	}
	return s.base.clock()()
}

// skipEmptyContent records a send suppressed by
// [mailtemplate.Template.RequireVisibleContent].
//
// Info, not Warn: suppression is the operator's configured intent, so nothing
// is broken and no action is available. It logs at all so an operator asking
// "why did this recipient get no mail?" can raise the level and find out —
// distinguishing "no matching data" from "no data this recipient may see".
//
// Only the template and recipient are named. What was filtered must never
// appear here: the whole point is that this recipient may not see it.
//
// Returns nil, matching [skipBadAddress] — a suppressed send is a completed
// job, not a failure. Template child jobs are enqueued RetryBounded
// (internal/scheduler/foreach.go), so returning an error would re-render on
// every attempt and never succeed.
func skipEmptyContent(ctx context.Context, template, recipientID string) error {
	slog.InfoContext(ctx, "scheduled mail has no visible content; skipping",
		"template", template, "recipient", recipientID)
	return nil
}

func skipBadAddress(ctx context.Context, recipientID, property string) error {
	slog.WarnContext(ctx, "scheduled mail recipient has no usable address; skipping",
		"recipient", recipientID, "property", property)
	return nil
}

// defaultWorldRow reads the raw row the default world selects for id. found
// is false when the world selects none.
//
// Raw, not through the task principal's visibility reader: the row only
// addresses the envelope and is never rendered or returned to the principal,
// and a recipient must be reachable even when the principal may not read the
// face that holds the address.
func (w *SchedulerWorkspace) defaultWorldRow(
	ctx context.Context, id string,
) (row *entity.Entity, found bool, err error) {
	q := store.EntityQuery{IDs: []string{id}, Faces: store.InWorld(w.worlds.DefaultWorld())}
	for e, lerr := range w.store.ListEntities(ctx, q) {
		if lerr != nil {
			return nil, false, lerr
		}
		if e.ID == id {
			return e, true, nil
		}
	}
	return nil, false, nil
}
