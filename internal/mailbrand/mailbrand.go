// Package mailbrand reads the app's look for rendered mail: the accent color
// of the configured palette, and a public logo address.
//
// It answers one question for every caller that builds a
// [mailrender.Renderer] (the Lua mail.render binding and the scheduled-mail
// templates), so mail matches the app the operator styled instead of rela's
// default blue.
//
// # What it reads
//
//   - The accent color, for the button and the top bar. It resolves the
//     data-entry.yaml `palette:` under the palette saved on the Settings page,
//     the same layering the web app uses, so mail and app agree. The mail's
//     neutral tones (card, text, borders) stay mailrender's: they are tuned for
//     mail clients, and the app's surface colors are not.
//   - `app.logo` from data-entry.yaml, made absolute by the caller's base URL.
//     That address is public by design (it serves the pages shown before a
//     workspace loads), so a mail client can fetch it. The logo uploaded on the
//     Settings page is served behind the session and is NOT used: a mail client
//     would show a broken image.
//   - `app.name`, as the logo's alt text.
//
// # Fallback
//
// Every missing piece falls back to mailrender's defaults: no palette means
// rela's accent, no logo (or an SVG one, which mail clients do not show) means
// no logo. A file that cannot be read or parsed is an error, so a caller can log
// it and render with the defaults rather than fail the mail.
package mailbrand

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// ConfigReader reads a project config file by name; config.Loader satisfies
// it. An absent file gives an os.IsNotExist error.
type ConfigReader interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// KVReader reads a value of the project's state store; state.KV satisfies it.
// An absent key gives an os.IsNotExist error.
type KVReader interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// Source reads the branding of one project. The zero value has no inputs and
// returns the defaults.
type Source struct {
	// Config reads data-entry.yaml. Nil: no app palette, logo or name.
	Config ConfigReader
	// KV reads the palette saved on the Settings page. Nil: none.
	KV KVReader
}

// configDoc is the part of data-entry.yaml this package reads.
type configDoc struct {
	App struct {
		Name string `yaml:"name"`
		Logo string `yaml:"logo"`
	} `yaml:"app"`
	Palette *dataentryconfig.PaletteConfig `yaml:"palette"`
}

// RenderOptions returns the renderer options for mail: the base URL the caller
// passes, with the app's accent, logo and name. On an error the options are
// still usable: they hold whatever could be read, and the defaults for the
// rest.
func (s Source) RenderOptions(ctx context.Context, baseURL string) (mailrender.Options, error) {
	doc, cfgErr := s.readConfig(ctx)
	user, userErr := s.readUserPalette(ctx)

	opts := mailrender.Options{BaseURL: baseURL, LogoAlt: doc.App.Name}
	if mailrender.ValidateLogoURL(doc.App.Logo) == nil {
		opts.LogoURL = doc.App.Logo
	}
	if doc.Palette != nil || user != nil {
		opts.Palette = accentTokens(dataentryconfig.ResolvePalette(doc.Palette, user))
	}
	return opts, errors.Join(cfgErr, userErr)
}

func (s Source) readConfig(ctx context.Context) (configDoc, error) {
	var doc configDoc
	if s.Config == nil {
		return doc, nil
	}
	data, err := s.Config.Load(ctx, dataentryconfig.ConfigFile)
	if isNotExist(err) {
		return doc, nil
	}
	if err != nil {
		return doc, fmt.Errorf("mailbrand: read %s: %w", dataentryconfig.ConfigFile, err)
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return configDoc{}, fmt.Errorf("mailbrand: parse %s: %w", dataentryconfig.ConfigFile, err)
	}
	if err := dataentryconfig.ValidatePalette(doc.Palette); err != nil {
		doc.Palette = nil
		return doc, fmt.Errorf("mailbrand: %s: %w", dataentryconfig.ConfigFile, err)
	}
	return doc, nil
}

//nolint:nilnil // (nil, nil) means "no palette saved on the Settings page"
func (s Source) readUserPalette(ctx context.Context) (*dataentryconfig.PaletteConfig, error) {
	if s.KV == nil {
		return nil, nil
	}
	data, err := s.KV.Get(ctx, dataentryconfig.UserPaletteKey)
	if isNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mailbrand: read %s: %w", dataentryconfig.UserPaletteKey, err)
	}
	var p dataentryconfig.PaletteConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("mailbrand: parse %s: %w", dataentryconfig.UserPaletteKey, err)
	}
	if err := dataentryconfig.ValidatePalette(&p); err != nil {
		return nil, fmt.Errorf("mailbrand: %s: %w", dataentryconfig.UserPaletteKey, err)
	}
	return &p, nil
}

// accentTokens maps the resolved palette's accent to mailrender's tokens. The
// dark accent is the app's dark accent, or the light one when the app has dark
// mode off. A value mailrender would refuse is left out, so the default shows.
func accentTokens(p *dataentryconfig.ResolvedPalette) map[string]string {
	const accent = "--accent-color"
	out := map[string]string{}
	light := p.Light[accent]
	dark := light
	if !p.DarkDisabled && p.Dark[accent] != "" {
		dark = p.Dark[accent]
	}
	for token, v := range map[string]string{"--accent-color": light, "--dark-accent-color": dark} {
		if v != "" && mailrender.ValidatePalette(map[string]string{token: v}) == nil {
			out[token] = v
		}
	}
	return out
}

func isNotExist(err error) bool {
	return err != nil && (errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err))
}
