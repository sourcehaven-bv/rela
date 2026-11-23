package dataentryconfig

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AboutConfig is `app.about:` in data-entry.yaml: what the About dialog in
// the sidebar footer shows besides the app name and description. Both keys
// are optional. The About button shows when the description, the version or
// a link is set, so a deployment with none of them has no empty dialog.
//
// It is display data the operator writes for every user, served on
// `/api/v1/_config` like the rest of the configuration. rela adds no build
// version of its own: what the operator puts here is what users see.
type AboutConfig struct {
	// Version is a free-text version of the deployment, such as "2026.11"
	// or a release name. Empty shows no version line.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	// Links are shown in order, each opening in a new tab: help pages,
	// terms, a privacy statement, a support address.
	Links []AboutLink `yaml:"links,omitempty" json:"links,omitempty"`
}

// AboutLink is one link of [AboutConfig]. URL is a root-relative path or an
// https URL, checked at load like the account links.
type AboutLink struct {
	Label string `yaml:"label" json:"label"`
	URL   string `yaml:"url" json:"url"`
}

// IsZero reports whether nothing is configured, so `omitzero` leaves an
// unset block out of the JSON.
func (a AboutConfig) IsZero() bool {
	return a.Version == "" && len(a.Links) == 0
}

// validateAbout checks `app.about:`: every link has a label and a URL that
// is a path on this host or an https URL ([ValidAccountLink]).
func validateAbout(a AboutConfig) []string {
	var errs []string
	for i, l := range a.Links {
		if strings.TrimSpace(l.Label) == "" {
			errs = append(errs, fmt.Sprintf("app.about.links[%d]: label is required", i))
		}
		if validAccountLink(l.URL) != nil {
			errs = append(errs, fmt.Sprintf("app.about.links[%d].url: %q must be a path starting with / "+
				"or an https:// URL", i, l.URL))
		}
	}
	return errs
}

// aboutKeys and aboutLinkKeys are the keys `app.about:` and each of its
// links accept.
var (
	aboutKeys     = []string{"version", "links"}
	aboutLinkKeys = []string{"label", "url"}
)

// aboutUnknownKeys reports keys under `app.about:` that it does not define.
// The decoder ignores unknown nested keys, so a typo such as `vers:`
// would silently show nothing.
func aboutUnknownKeys(data []byte) []string {
	var raw struct {
		App struct {
			About map[string]any `yaml:"about"`
		} `yaml:"app"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	errs := unknownKeysIn("app.about", raw.App.About, aboutKeys)
	return append(errs, unknownKeysInList("app.about.links", raw.App.About["links"], aboutLinkKeys)...)
}
