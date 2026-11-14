package scalerun

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the parsed scalerun.yaml.
type Config struct {
	// Iterations is the number of measured requests per endpoint and
	// principal. Warmup requests run first and are not measured.
	Iterations int `yaml:"iterations"`
	Warmup     int `yaml:"warmup"`
	// Principals maps a short role name to the identity sent in the
	// principal header. Endpoints refer to the short name.
	Principals map[string]string `yaml:"principals"`
	// FixturesAs is the principal the fixture ids are read as. It must read
	// tasks, projects and people.
	FixturesAs string      `yaml:"fixtures_as"`
	Ratio      RatioConfig `yaml:"ratio"`
	Bench      BenchConfig `yaml:"bench"`
	Endpoints  []Endpoint  `yaml:"endpoints"`
}

// RatioConfig bounds how much slower an endpoint may get when the data
// grows from the base scale to a larger one.
type RatioConfig struct {
	// BaseScale is the scale the others are compared with.
	BaseScale float64 `yaml:"base_scale"`
	// Max is the largest allowed p50(scale)/p50(base).
	Max float64 `yaml:"max"`
	// FloorMS skips the check when the larger scale's p50 is below it:
	// at a few milliseconds the ratio measures noise, not growth.
	FloorMS float64 `yaml:"floor_ms"`
}

// BenchConfig bounds benchmark regressions against the previous run.
type BenchConfig struct {
	// MaxRegression is the largest allowed slowdown as a fraction (0.2 is
	// 20%). Only a change benchstat reports as significant counts.
	MaxRegression float64 `yaml:"max_regression"`
}

// Endpoint is one request shape the driver measures.
//
// Path and Body are templates. The placeholders are filled per request:
// {task}, {project}, {person} and {word} from the fixtures the driver reads
// at startup; {new} mints a fresh task id and remembers it; {created} takes
// the oldest id {new} minted and not yet taken; {priority} cycles through
// the task priorities so an update is never a no-op.
type Endpoint struct {
	Name   string   `yaml:"name"`
	Method string   `yaml:"method"`
	Path   string   `yaml:"path"`
	Body   string   `yaml:"body"`
	As     []string `yaml:"as"`
	Expect int      `yaml:"expect"`
	// P95MS maps a scale (as written in the run, e.g. "1" or "5") to the
	// p95 limit in milliseconds for every principal.
	P95MS map[string]float64 `yaml:"p95_ms"`
	// RatioMax overrides Ratio.Max for this endpoint. Say why in a comment
	// next to it in the config.
	RatioMax float64 `yaml:"ratio_max"`
}

var endpointName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// LoadConfig reads and validates a config file.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(raw)
}

// ParseConfig parses and validates config YAML.
func ParseConfig(raw []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	var errs []error
	if c.Iterations < 2 {
		errs = append(errs, errors.New("iterations must be at least 2"))
	}
	if c.Warmup < 0 {
		errs = append(errs, errors.New("warmup must not be negative"))
	}
	if len(c.Principals) == 0 {
		errs = append(errs, errors.New("principals must not be empty"))
	}
	if _, ok := c.Principals[c.FixturesAs]; !ok {
		errs = append(errs, fmt.Errorf("fixtures_as %q is not a principal", c.FixturesAs))
	}
	if c.Ratio.BaseScale <= 0 || c.Ratio.Max <= 1 {
		errs = append(errs, errors.New("ratio needs base_scale > 0 and max > 1"))
	}
	if c.Bench.MaxRegression <= 0 {
		errs = append(errs, errors.New("bench.max_regression must be positive"))
	}
	seen := map[string]bool{}
	minted := 0
	for i, e := range c.Endpoints {
		where := fmt.Sprintf("endpoint %d (%s)", i, e.Name)
		if !endpointName.MatchString(e.Name) || seen[e.Name] {
			errs = append(errs, fmt.Errorf("%s: name must be unique and match %s", where, endpointName))
		}
		seen[e.Name] = true
		errs = append(errs, c.validateEndpoint(e, where)...)
		// {created} consumes what {new} minted, so a take must follow a
		// mint of at least as many requests.
		n := len(e.As)
		for _, ph := range []string{"{new}", "{created}"} {
			if strings.Contains(e.Path, ph) && strings.Contains(e.Body, ph) {
				errs = append(errs, fmt.Errorf("%s: %s may appear in the path or the body, not both", where, ph))
			}
		}
		if usesPlaceholder(e, "{new}") {
			minted += n
		}
		if usesPlaceholder(e, "{created}") {
			if minted < n {
				errs = append(errs, fmt.Errorf("%s: {created} needs an earlier endpoint using {new} "+
					"for at least as many principals", where))
			}
			minted -= n
		}
	}
	if len(c.Endpoints) == 0 {
		errs = append(errs, errors.New("endpoints must not be empty"))
	}
	return errors.Join(errs...)
}

// validateEndpoint checks the fields of e that need no other endpoint.
func (c *Config) validateEndpoint(e Endpoint, where string) []error {
	var errs []error
	if !slices.Contains([]string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}, e.Method) {
		errs = append(errs, fmt.Errorf("%s: method %q is not GET, POST, PATCH or DELETE", where, e.Method))
	}
	if !strings.HasPrefix(e.Path, "/") {
		errs = append(errs, fmt.Errorf("%s: path must start with /", where))
	}
	if e.Expect < 100 || e.Expect > 599 {
		errs = append(errs, fmt.Errorf("%s: expect must be an HTTP status", where))
	}
	if len(e.As) == 0 {
		errs = append(errs, fmt.Errorf("%s: as must name at least one principal", where))
	}
	for _, p := range e.As {
		if _, ok := c.Principals[p]; !ok {
			errs = append(errs, fmt.Errorf("%s: unknown principal %q", where, p))
		}
	}
	for scale, ms := range e.P95MS {
		if _, err := strconv.ParseFloat(scale, 64); err != nil || ms <= 0 {
			errs = append(errs, fmt.Errorf("%s: p95_ms needs numeric scales and positive limits", where))
		}
	}
	if e.RatioMax != 0 && e.RatioMax <= 1 {
		errs = append(errs, fmt.Errorf("%s: ratio_max must be above 1", where))
	}
	return errs
}

// ratioMax is the ratio limit for e.
func (c *Config) ratioMax(e Endpoint) float64 {
	if e.RatioMax != 0 {
		return e.RatioMax
	}
	return c.Ratio.Max
}

// endpoint returns the endpoint named name.
func (c *Config) endpoint(name string) (Endpoint, bool) {
	for _, e := range c.Endpoints {
		if e.Name == name {
			return e, true
		}
	}
	return Endpoint{}, false
}

// FormatScale writes a scale the way the config's p95_ms keys and the
// report spell it: 1, 5, 0.5.
func FormatScale(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64)
}
