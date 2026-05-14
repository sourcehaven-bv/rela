package dataentryconfig

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// init asserts at startup that ThemeManifest's top-level keys don't
// collide with any inlined PaletteConfig key. yaml.v3 *panics* on
// duplicate-key shadow during Unmarshal — catching this at boot
// turns a runtime-fatal regression into a startup error.
func init() {
	if err := checkManifestTagsUnique(); err != nil {
		panic("dataentryconfig: theme manifest tag collision: " + err.Error())
	}
}

func checkManifestTagsUnique() error {
	mt := reflect.TypeFor[ThemeManifest]()
	seen := make(map[string]string)
	var visit func(t reflect.Type, path string) error
	visit = func(t reflect.Type, path string) error {
		for i := range t.NumField() {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("yaml"), ",")
			name := strings.TrimSpace(tag[0])
			inline := false
			for _, opt := range tag[1:] {
				if strings.TrimSpace(opt) == "inline" {
					inline = true
				}
			}
			if inline {
				if err := visit(f.Type, path+"."+f.Name); err != nil {
					return err
				}
				continue
			}
			if name == "" || name == "-" {
				continue
			}
			if prev, dup := seen[name]; dup {
				return fmt.Errorf("yaml key %q appears in both %s and %s%s", name, prev, path, "."+f.Name)
			}
			seen[name] = path + "." + f.Name
		}
		return nil
	}
	return visit(mt, "ThemeManifest")
}

// ThemeManifest is the metadata + palette payload of a `.relatheme`
// theme package. It embeds PaletteConfig so a manifest is a superset
// of the existing `palette.yaml` shape — anything that's a valid
// palette overlay is, with the addition of name + version, a valid
// theme manifest.
//
// Logo, when set, references a flat-file zip entry of the form
// `logo.<ext>` (e.g. `logo.png`). The manifest never embeds the bytes
// itself; the importer reads them from the corresponding zip entry.
type ThemeManifest struct {
	Name    string `yaml:"name"               json:"name"`
	Version string `yaml:"version"            json:"version"`
	Author  string `yaml:"author,omitempty"   json:"author,omitempty"`
	Logo    string `yaml:"logo,omitempty"     json:"logo,omitempty"`

	// Palette colors are inlined at the top level so a manifest reads
	// like a superset of palette.yaml.
	PaletteConfig `yaml:",inline" json:",inline"`
}

// Manifest length limits. These are deliberately generous — a theme
// description is metadata, not user content.
const (
	maxThemeNameLen    = 100
	maxThemeVersionLen = 32
	maxThemeAuthorLen  = 100
)

// ValidateThemeManifest checks the shape of a parsed manifest. It
// re-uses ValidatePalette for the embedded palette fields so manifest
// validation tracks palette validation by construction.
func ValidateThemeManifest(m *ThemeManifest) error {
	if m == nil {
		return errors.New("manifest is nil")
	}
	if l := len(m.Name); l < 1 || l > maxThemeNameLen {
		return fmt.Errorf("name must be 1-%d chars (got %d)", maxThemeNameLen, l)
	}
	if l := len(m.Version); l < 1 || l > maxThemeVersionLen {
		return fmt.Errorf("version must be 1-%d chars (got %d)", maxThemeVersionLen, l)
	}
	if l := len(m.Author); l > maxThemeAuthorLen {
		return fmt.Errorf("author must be 0-%d chars (got %d)", maxThemeAuthorLen, l)
	}
	if m.Logo != "" {
		if err := validateLogoEntryName(m.Logo); err != nil {
			return fmt.Errorf("logo: %w", err)
		}
	}
	return ValidatePalette(&m.PaletteConfig)
}

// allowedLogoEntryExts is the set of extensions a manifest may
// declare for its logo entry. Importers re-sniff the actual bytes via
// http.DetectContentType, so the manifest's extension is technically
// just a lookup key — but rejecting unknown extensions at validation
// time keeps the manifest's stated shape honest with what we'll
// actually persist.
var allowedLogoEntryExts = map[string]struct{}{
	"png":  {},
	"jpeg": {},
	"jpg":  {}, // alias for jpeg, accepted in manifests
	"svg":  {},
	"webp": {},
}

// validateLogoEntryName enforces the manifest's logo field shape. The
// importer trusts the actual sniffed mime regardless, but the manifest
// must still describe a flat filename within the format allowlist so a
// zip parser can locate the bytes without directory walks and so
// reading the manifest tells the truth about what kind of asset is
// expected.
func validateLogoEntryName(name string) error {
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf(`must be a flat filename, not a path (got %q)`, name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf(`must not contain ".." (got %q)`, name)
	}
	if !strings.HasPrefix(name, "logo.") {
		return fmt.Errorf(`must start with "logo." (got %q)`, name)
	}
	ext := strings.ToLower(name[len("logo."):])
	if ext == "" {
		return fmt.Errorf(`must have an extension after "logo." (got %q)`, name)
	}
	if _, ok := allowedLogoEntryExts[ext]; !ok {
		return fmt.Errorf(`unsupported extension %q (allowed: png, jpeg, jpg, svg, webp)`, ext)
	}
	return nil
}
