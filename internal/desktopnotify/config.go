package desktopnotify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// ConfigFile is the project-root file this package reads.
const ConfigFile = "desktop.yaml"

// defaultTitle is the title template of a rule that declares none.
const defaultTitle = "{{entity.title}}"

// ruleIDPattern restricts rule ids to characters that are safe in a state key
// and in a log line.
var ruleIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ConfigLoader reads a project-root config file. It is satisfied by
// config.Loader, which appbuild.Services.ProjectFiles returns. A missing file
// is an error satisfying errors.Is(err, fs.ErrNotExist).
type ConfigLoader interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// fileConfig is the YAML shape of desktop.yaml.
type fileConfig struct {
	Notifications []fileRule `yaml:"notifications"`
	Badge         *fileBadge `yaml:"badge"`
}

type fileRule struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	Condition string `yaml:"condition"`
	Title     string `yaml:"title"`
	Body      string `yaml:"body"`
}

type fileBadge struct {
	Type      string `yaml:"type"`
	Condition string `yaml:"condition"`
}

// Config is a loaded and compiled desktop.yaml. It is immutable after [Load]
// and safe for concurrent use.
type Config struct {
	meta  *metamodel.Metamodel
	eval  *predicatefns.Evaluator
	rules []rule
	badge *condition
}

// condition is one compiled (type, predicate) pair.
type condition struct {
	entityType string
	prog       *predicate.Program
}

type rule struct {
	id string
	condition
	title template
	body  template
}

// Option configures [Load].
type Option func(*options)

type options struct {
	now func() time.Time
}

// WithClock sets the clock that today() reads in conditions. The default is
// time.Now. A nil clock is ignored.
func WithClock(now func() time.Time) Option {
	return func(o *options) {
		if now != nil {
			o.now = now
		}
	}
}

// Load reads desktop.yaml through files and compiles it against meta.
//
// A missing file is not an error: it yields a Config with no rules and no
// badge. Every other problem fails the load: unreadable or malformed YAML, an
// unknown key, a missing or duplicate rule id, an unknown entity type, a
// condition that does not compile, or a template naming an unknown property.
func Load(
	ctx context.Context, files ConfigLoader, meta *metamodel.Metamodel, opts ...Option,
) (*Config, error) {
	if files == nil {
		return nil, errors.New("desktopnotify: Load requires a config loader")
	}
	if meta == nil {
		return nil, errors.New("desktopnotify: Load requires a metamodel")
	}
	o := options{now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	cfg := &Config{meta: meta, eval: predicatefns.NewEvaluatorWithClock(meta, o.now)}

	data, err := files.Load(ctx, ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ConfigFile, err)
	}

	var fc fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: %w", ConfigFile, err)
	}
	if err := cfg.compile(fc); err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	return cfg, nil
}

func (c *Config) compile(fc fileConfig) error {
	seen := make(map[string]bool, len(fc.Notifications))
	for i, fr := range fc.Notifications {
		r, err := c.compileRule(fr)
		if err != nil {
			return fmt.Errorf("notifications[%d]: %w", i, err)
		}
		if seen[r.id] {
			return fmt.Errorf("notifications[%d]: duplicate id %q", i, r.id)
		}
		seen[r.id] = true
		c.rules = append(c.rules, r)
	}
	if fc.Badge != nil {
		cond, err := c.compileCondition(fc.Badge.Type, fc.Badge.Condition)
		if err != nil {
			return fmt.Errorf("badge: %w", err)
		}
		c.badge = &cond
	}
	return nil
}

func (c *Config) compileRule(fr fileRule) (rule, error) {
	if fr.ID == "" {
		return rule{}, errors.New("id is required")
	}
	if !ruleIDPattern.MatchString(fr.ID) {
		return rule{}, fmt.Errorf("id %q must match [a-z0-9_-]+", fr.ID)
	}
	cond, err := c.compileCondition(fr.Type, fr.Condition)
	if err != nil {
		return rule{}, fmt.Errorf("rule %q: %w", fr.ID, err)
	}
	def, _ := c.meta.GetEntityDef(cond.entityType)
	titleSrc := fr.Title
	if titleSrc == "" {
		titleSrc = defaultTitle
	}
	title, err := parseTemplate(titleSrc, def)
	if err != nil {
		return rule{}, fmt.Errorf("rule %q: title: %w", fr.ID, err)
	}
	body, err := parseTemplate(fr.Body, def)
	if err != nil {
		return rule{}, fmt.Errorf("rule %q: body: %w", fr.ID, err)
	}
	return rule{id: fr.ID, condition: cond, title: title, body: body}, nil
}

func (c *Config) compileCondition(entityType, source string) (condition, error) {
	if entityType == "" {
		return condition{}, errors.New("type is required")
	}
	if source == "" {
		return condition{}, errors.New("condition is required")
	}
	if _, ok := c.meta.GetEntityDef(entityType); !ok {
		return condition{}, fmt.Errorf("unknown entity type %q", entityType)
	}
	// Resolve an alias once, so the store query and the evaluator agree.
	canonical := c.meta.ResolveAlias(entityType)
	prog, err := c.eval.Compile(canonical, source)
	if err != nil {
		// The desktop has no person mapping for its user, so the
		// current-user profile is never offered. Compiling against it
		// only tells the operator WHY the condition failed.
		if _, uerr := c.eval.CompileWithCurrentUser(canonical, source); uerr == nil {
			return condition{}, fmt.Errorf(
				"condition %q: current_user is not available in %s", source, ConfigFile)
		}
		return condition{}, fmt.Errorf("condition %q: %w", source, err)
	}
	if len(prog.Traversals()) > 0 {
		return condition{}, fmt.Errorf(
			"condition %q: related() is not available in %s", source, ConfigFile)
	}
	return condition{entityType: canonical, prog: prog}, nil
}

// RuleIDs returns the notification rule ids in declaration order.
func (c *Config) RuleIDs() []string {
	ids := make([]string, len(c.rules))
	for i, r := range c.rules {
		ids[i] = r.id
	}
	return ids
}

// HasBadge reports whether desktop.yaml declares a badge.
func (c *Config) HasBadge() bool { return c.badge != nil }
