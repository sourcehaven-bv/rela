// Package subscription imports syndication feeds (RSS, Atom, JSON Feed) into
// entities. An operator declares each feed in subscriptions.yaml with a URL, a
// polling interval and an ingest mapping; the scheduler runs one
// subscription per task, and [Run] does one poll.
//
// # One poll
//
// A poll is a conditional GET (ETag / Last-Modified), a parse through package
// feed, and one ingest.Run per item, newest items first in, oldest first out.
// State per subscription lives in state.KV: the validators and a bounded
// seen-set of item identities with a hash of their content, so an unchanged
// item costs no store read and a deleted import is not recreated while its
// item stays in the feed.
//
// # Trust
//
// The config is operator-authored and trusted. The feed is not: item HTML
// reaches an entity only as markdown converted by htmlmd (which sanitizes with
// bluemonday), every other value is flattened to one line, links keep only
// http and https, and redirects stay on the configured origin so the
// publisher cannot point the server, or the subscription's credentials,
// anywhere else.
package subscription

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// FileName is the subscriptions config in the project root.
const FileName = "subscriptions.yaml"

const (
	// DefaultMaxItems is how many of a feed's newest items one poll imports
	// when max_items is not set.
	DefaultMaxItems = 50
	// MaxMaxItems bounds max_items. Each new item scans the find type, so the
	// bound caps the store reads of one poll.
	MaxMaxItems = 1000
)

// Config is the parsed subscriptions.yaml.
type Config struct {
	Subscriptions map[string]Subscription `yaml:"subscriptions"`
}

// Subscription is one polled feed.
type Subscription struct {
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url"`

	// Every is the polling schedule in schedules.yaml syntax. The scheduler
	// parses it, so the syntax has one home.
	Every string `yaml:"every"`

	// MaxItems is how many of the newest items one poll imports. Zero means
	// [DefaultMaxItems].
	MaxItems int `yaml:"max_items,omitempty"`

	Auth *Auth `yaml:"auth,omitempty"`

	ingestspec.Mapping `yaml:",inline"`
}

// EffectiveMaxItems returns MaxItems or its default.
func (s Subscription) EffectiveMaxItems() int {
	if s.MaxItems == 0 {
		return DefaultMaxItems
	}
	return s.MaxItems
}

// Auth names the credential sent with each request. The secret is a key in
// .rela/secrets.yaml, never the value itself, so the config can be committed.
type Auth struct {
	// Type is bearer (Authorization: Bearer), basic (Authorization: Basic with
	// Username) or header (the secret as the value of Header).
	Type     string `yaml:"type"`
	Secret   string `yaml:"secret"`
	Username string `yaml:"username,omitempty"`
	Header   string `yaml:"header,omitempty"`
}

// Auth types.
const (
	AuthBearer = "bearer"
	AuthBasic  = "basic"
	AuthHeader = "header"
)

// nameRegex is the webhook id rule: the name becomes part of a principal
// (system:subscription:<name>) and of a task name, so it stays plain.
var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// headerNameRegex is the RFC 7230 token grammar.
var headerNameRegex = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+._` + "`" + `|~^-]{1,128}$`)

// refs is the closed set of references a subscription template may use. A
// webhook body is open-ended, but a feed item has a fixed shape, so a typo
// such as item.guid is a load error rather than a value that is always empty.
var refs = map[string]bool{
	"item.id": true, "item.title": true, "item.link": true,
	"item.summary": true, "item.content": true,
	"item.published": true, "item.published_date": true, "item.updated": true,
	"item.authors": true, "item.categories": true,
	"feed.title": true, "feed.link": true,
	"subscription.name": true,
	"now":               true, "today": true,
}

// blockRefs are the references that resolve to a multi-line markdown block.
var blockRefs = map[string]bool{"item.summary": true, "item.content": true}

// Parse decodes and validates subscriptions.yaml. Every problem is reported,
// joined into one error.
//
// Nil: accepted for meta; types and properties are then reported unknown,
// which is what a config check without a schema can say.
func Parse(data []byte, meta *metamodel.Metamodel) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	var errs []error
	for _, name := range cfg.Names() {
		for _, msg := range cfg.Subscriptions[name].validate(meta) {
			errs = append(errs, fmt.Errorf("subscriptions: %q %s", name, msg))
		}
		if !nameRegex.MatchString(name) {
			errs = append(errs, fmt.Errorf(
				"subscriptions: invalid name %q (must match %s)", name, nameRegex.String()))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Names returns the subscription names in sorted order.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Subscriptions))
	for name := range c.Subscriptions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s Subscription) validate(meta *metamodel.Metamodel) []string {
	var errs []string
	if err := checkURL(s.URL); err != nil {
		errs = append(errs, err.Error())
	}
	if s.Every == "" {
		errs = append(errs, "every is required")
	}
	if s.MaxItems < 0 || s.MaxItems > MaxMaxItems {
		errs = append(errs, fmt.Sprintf("max_items %d must be between 0 (the default, %d) and %d", s.MaxItems, DefaultMaxItems, MaxMaxItems))
	}
	if s.Auth != nil {
		errs = append(errs, s.Auth.validate()...)
		// A credential over plain http is readable by anyone on the path,
		// on every poll and before any https redirect.
		if !strings.HasPrefix(strings.ToLower(s.URL), "https://") {
			errs = append(errs, "auth requires an https url")
		}
	}

	errs = append(errs, s.Validate(meta)...)
	// The entity, not the seen-set, holds an item's identity: the seen-set
	// is bounded and is reset when the config changes, so a mapping that
	// always creates would duplicate every item it forgot.
	if s.Find == nil || len(s.Find.Match) == 0 {
		errs = append(errs, "needs find.match: the entity holds the item identity, so a later poll finds it")
	}
	return append(errs, s.validateRefs()...)
}

// checkURL allows an absolute http or https URL with a host and no userinfo.
// Credentials belong in auth, where they come from secrets.yaml.
func checkURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("url does not parse")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must be http or https")
	}
	if u.Hostname() == "" {
		return errors.New("url needs a host")
	}
	if u.User != nil {
		return errors.New("url must not carry credentials; use auth with a secret")
	}
	if u.Fragment != "" {
		return errors.New("url must not carry a fragment")
	}
	return nil
}

func (a *Auth) validate() []string {
	var errs []string
	switch a.Type {
	case AuthBearer, AuthBasic, AuthHeader:
	default:
		errs = append(errs, fmt.Sprintf("auth.type %q must be one of bearer, basic, header", a.Type))
	}
	switch a.Secret {
	case "":
		errs = append(errs, "auth.secret names no secret")
	case secrets.TokenKey:
		errs = append(errs, fmt.Sprintf("auth.secret may not be %q, which signs rela's own tokens", secrets.TokenKey))
	}
	if (a.Type == AuthBasic) != (a.Username != "") {
		errs = append(errs, "auth.username is required for basic auth and only allowed there")
	}
	if a.Type == AuthHeader {
		if !headerNameRegex.MatchString(a.Header) {
			errs = append(errs, fmt.Sprintf("auth.header %q is not a valid HTTP header name", a.Header))
		}
	} else if a.Header != "" {
		errs = append(errs, "auth.header is only allowed for type header")
	}
	return errs
}

// validateRefs checks every template against the closed reference set, and
// refuses block content combined with append_section.
func (s Subscription) validateRefs() []string {
	var errs []string
	bodyHasBlock := false
	s.Templates(func(path, tmpl string) {
		for _, ref := range templateRefs(tmpl) {
			if !refs[ref] {
				errs = append(errs, fmt.Sprintf("%s uses unknown reference {{%s}}", path, ref))
			}
			if path == "create_if_missing.content" && blockRefs[ref] {
				bodyHasBlock = true
			}
		}
	})
	if s.Find != nil {
		for _, prop := range s.Find.Match {
			if _, ok := s.Find.Values[prop]; !ok && !refs["item."+prop] {
				errs = append(errs, fmt.Sprintf(
					"find.match %q has no find.values entry and item.%s is not an item field", prop, prop))
			}
		}
	}
	// Feed HTML converted to markdown keeps its headings. In a new body they
	// would become sections an append_section step could then land in,
	// which is the forgery flattening exists to stop.
	if bodyHasBlock && slices.ContainsFunc(s.Then, func(st ingestspec.Step) bool { return st.AppendSection != nil }) {
		errs = append(errs, "uses item.summary or item.content in create_if_missing.content together with "+
			"append_section; feed headings could forge the section")
	}
	return errs
}

// templateRefs returns the trimmed references in tmpl, in the same way
// ingest.Interpolate finds them.
func templateRefs(tmpl string) []string {
	var out []string
	rest := tmpl
	for {
		_, after, found := strings.Cut(rest, "{{")
		if !found {
			return out
		}
		ref, remainder, closed := strings.Cut(after, "}}")
		if !closed {
			return out
		}
		out = append(out, strings.TrimSpace(ref))
		rest = remainder
	}
}
