// Package remotesync pulls rows from another rela instance, a sync remote, into
// this one (FEAT-CMVZQFI, TKT-I8L1EQU). An operator declares each remote in
// remotes.yaml: its address, where the credential this instance presents to
// it comes from, which of its types to pull, and how their fields map onto
// local types.
//
// # Identity
//
// rela issues no credentials of its own (DEC-QKLMMD4). On the remote, this
// instance is an ordinary principal: the credential is whatever the
// remote's identity edge accepts, such as a machine-client token from an
// OIDC proxy, and the remote's acl.yaml decides what it may read. Here, a
// pull writes as the local principal that runs it: the CLI user, or the
// task identity the scheduler binds.
//
// # One pull
//
// [Run] compares the source with a ledger of what earlier pulls applied.
// The ledger is the local side of a reconcile diff, so the diff names the
// rows the source added, changed and no longer lists. The run fetches the
// added and changed rows through the source's read routes, merges the
// fields the source owns into the local rows, and writes through the entity
// manager. It never picks a local id: a new row gets a generated id and an
// external ref that holds the source id.
//
// # Trust
//
// The config is operator-authored and trusted. The source is not: its
// answers are capped, a fetched row must be the row asked for, only mapped
// fields are read, every value is coerced to the local field type, and the
// manager validates the write. A run that would remove many rows at once is
// refused before it writes anything (the gone guard).
package remotesync

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// FileName is the remotes config in the project root.
const FileName = "remotes.yaml"

// The gone policies.
const (
	// OnGoneDetach keeps a local row whose source row is gone. It is the
	// default.
	OnGoneDetach = "detach"
	// OnGoneDelete deletes such a row when nothing local depends on it.
	OnGoneDelete = "delete"
)

// The conflict rules (DEC-2NFLAHB). A rule decides a field both sides
// changed since the last sync. It is set per remote, per type and per field;
// the most specific one applies.
const (
	// ConflictHold changes neither side and puts the field on the
	// conflict list until someone resolves it. It is the default.
	ConflictHold = "hold"
	// ConflictLatest takes the value of the side that changed the row last,
	// by the last-updated times both sides serve, and keeps the losing local
	// value on the conflict list. It holds instead when a time is withheld,
	// not trusted, or the same on both sides.
	ConflictLatest = "latest"
	// ConflictLocal keeps the local value and sends it to the remote.
	ConflictLocal = "local"
	// ConflictRemote takes the remote value.
	ConflictRemote = "remote"
)

// DefaultMaxClockSkew is how far a last-updated time may lie in the future
// before latest stops trusting it. MaxMaxClockSkew bounds the setting.
const (
	DefaultMaxClockSkew = 5 * time.Minute
	MaxMaxClockSkew     = time.Hour
)

// DefaultSupersededRetention is how long a superseded entry stays on the
// conflict list when superseded_retention is not set.
const DefaultSupersededRetention = 30 * 24 * time.Hour

// Defaults and bounds of the per-remote limits.
const (
	DefaultMaxGone  = 10
	DefaultMaxRows  = 100_000
	MaxMaxRows      = 1_000_000
	DefaultMaxFetch = 10_000
	MaxMaxFetch     = 100_000
	DefaultMaxBytes = 256 << 20
	MaxMaxBytes     = 4 << 30
)

// Config is the parsed remotes.yaml.
type Config struct {
	Remotes map[string]Remote `yaml:"remotes"`
	// Mappings holds each remote's compiled mapping, by remote name. [Parse]
	// sets it.
	Mappings map[string]*Mapping `yaml:"-"`
}

// Remote is one source this instance pulls from.
type Remote struct {
	Description string `yaml:"description,omitempty"`
	// URL is the source's base address, as for `rela reconcile --remote`.
	URL string `yaml:"url"`
	// Credential names where the credential presented to the remote comes
	// from: exactly one of an environment variable, a file, or a global
	// secret under remote_credential. The value never appears here.
	Credential httpremote.Credential `yaml:"credential"`
	// AuthHeader is the request header that carries the credential. Empty
	// means Authorization, which sends it as a bearer token.
	AuthHeader string `yaml:"auth_header,omitempty"`
	// RunAs is the identity a scheduled pull runs and writes as. Empty means
	// the scheduler's own identity, as for a scheduled task. A pull by hand
	// runs as the person who starts it.
	RunAs string `yaml:"run_as,omitempty"`
	// Every is the pull schedule in schedules.yaml syntax. Empty means the
	// remote is pulled only by `rela remote pull`.
	Every string `yaml:"every,omitempty"`
	// AllowInsecureHTTP permits plain http to a host that is not loopback.
	AllowInsecureHTTP bool `yaml:"allow_insecure_http,omitempty"`
	// OnGone is [OnGoneDetach] (the default) or [OnGoneDelete].
	OnGone string `yaml:"on_gone,omitempty"`
	// MaxGone is how many rows one run may detach or delete before it is
	// refused. Nil means [DefaultMaxGone]; zero refuses any.
	MaxGone *int `yaml:"max_gone,omitempty"`
	// MaxRows caps the rows in scope on either side. Zero means
	// [DefaultMaxRows].
	MaxRows int `yaml:"max_rows,omitempty"`
	// MaxFetch caps the rows one run fetches; the rest wait for the next
	// run. Zero means [DefaultMaxFetch].
	MaxFetch int `yaml:"max_fetch,omitempty"`
	// MaxBytes caps the bytes one run reads in fetched rows; the rest wait
	// for the next run. Zero means [DefaultMaxBytes].
	MaxBytes int64 `yaml:"max_bytes,omitempty"`
	// Push makes the remote two-way: after each pull, the run pushes the
	// local changes to mapped fields, and the shared local rows the remote
	// does not have yet, through the remote's write API (DEC-2NFLAHB).
	Push bool `yaml:"push,omitempty"`
	// Conflict is the rule for a field both sides changed since the last
	// sync. Empty means [ConflictHold].
	Conflict string `yaml:"conflict,omitempty"`
	// MaxClockSkew is how far in the future a last-updated time may lie
	// before the latest rule holds instead of trusting it, as a Go duration
	// such as "5m". Empty means [DefaultMaxClockSkew].
	MaxClockSkew string `yaml:"max_clock_skew,omitempty"`
	// SupersededRetention is how long a superseded entry stays on the
	// conflict list, as a Go duration or a whole number of days such as
	// "30d". Empty means [DefaultSupersededRetention]. Zero keeps no losing
	// local value at all; the entry then holds only digests and is not
	// dropped by age.
	SupersededRetention string `yaml:"superseded_retention,omitempty"`
	// Entities maps source entity types, by name, to local ones.
	Entities map[string]EntityMap `yaml:"entities"`
	// Relations maps source relation types, by name, to local ones.
	Relations map[string]RelationMap `yaml:"relations,omitempty"`
}

// EntityMap maps one source entity type.
type EntityMap struct {
	// Type is the local type; empty means the source type's name.
	Type string `yaml:"type,omitempty"`
	// Ref is the local external_ref property that holds the source id. Its
	// system belongs to this remote alone.
	Ref string `yaml:"ref"`
	// Fields maps a local field to the source field it is pulled from. The
	// source owns these fields; every other local field is local. On a
	// remote with push, they flow both ways.
	Fields map[string]string `yaml:"fields,omitempty"`
	// Share is a condition on the local row, in the predicate language,
	// that a row must meet to be pushed. Empty shares every row of the
	// type. Only a remote with push takes it.
	Share string `yaml:"share,omitempty"`
	// Conflict overrides the remote's conflict rule for this type.
	Conflict string `yaml:"conflict,omitempty"`
	// FieldConflicts overrides the conflict rule per local field.
	FieldConflicts map[string]string `yaml:"field_conflicts,omitempty"`
}

// RelationMap maps one source relation type.
type RelationMap struct {
	// Type is the local type; empty means the source type's name.
	Type string `yaml:"type,omitempty"`
	// Fields maps local relation meta to source meta; "content" is the
	// body. On a remote with push, meta flows both ways under the conflict
	// rules; the body always comes from the source.
	Fields map[string]string `yaml:"fields,omitempty"`
	// Conflict overrides the remote's conflict rule for this type.
	Conflict string `yaml:"conflict,omitempty"`
	// FieldConflicts overrides the conflict rule per local meta field.
	FieldConflicts map[string]string `yaml:"field_conflicts,omitempty"`
}

// Parse decodes and validates remotes.yaml against meta. Every problem is
// reported, joined into one error.
//
// Nil: rejected for meta; a mapping cannot be checked without the schema.
func Parse(data []byte, meta *metamodel.Metamodel) (*Config, error) {
	if meta == nil {
		return nil, fmt.Errorf("parse %s: no schema to check it against", FileName)
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	var errs []error
	systems := map[string]string{}
	cfg.Mappings = make(map[string]*Mapping, len(cfg.Remotes))
	for _, name := range cfg.Names() {
		m, err := Compile(name, cfg.Remotes[name], meta)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if other, taken := systems[m.System]; taken {
			errs = append(errs, fmt.Errorf("remotes: %q: ref system %q is also used by remote %q",
				name, m.System, other))
			continue
		}
		systems[m.System] = name
		cfg.Mappings[name] = m
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Names returns the remote names in sorted order.
func (c *Config) Names() []string { return sortedKeys(c.Remotes) }

// Mapping is one remote's config checked against the local schema.
type Mapping struct {
	Name   string
	Remote Remote
	// System is the external-ref system of every ref property.
	System string
	// Entities and Relations are keyed by source type.
	Entities  map[string]*EntityMapping
	Relations map[string]*RelationMapping
	// Hash identifies the mapping. A ledger written under another hash is
	// compared again in full.
	Hash string

	// eval matches the share conditions.
	eval *predicatefns.Evaluator
}

// EntityMapping maps one source entity type.
type EntityMapping struct {
	Source, Local, Ref string
	Fields             []FieldMapping
	// Share selects the local rows a push sends; nil shares every row.
	Share *predicate.Program
}

// RelationMapping maps one source relation type.
type RelationMapping struct {
	Source, Local string
	Fields        []FieldMapping
}

// FieldMapping is one owned field: the local field, typed, the source field
// it is read from, and the conflict rule that applies to it.
type FieldMapping struct {
	Local  syncmerge.Field
	Remote string
	// Rule is the field's conflict rule, resolved from the field, type and
	// remote settings. It is never empty after [Compile].
	Rule string
}

// baseKey names the field in a ledger row's base. It holds both names, so a
// field mapped to another source field starts without a base.
func (f FieldMapping) baseKey() string { return baseKeyOf(f.Local.Name, f.Remote) }

// toContent reports whether the field maps the local body to the remote
// body. Pull and push both route by it, so a value travels the same way in
// each direction; any other mapping is a property on the remote side.
func (f FieldMapping) toContent() bool {
	return f.Remote == syncmerge.ContentField && f.Local.Kind == syncmerge.KindContent
}

func baseKeyOf(local, remote string) string { return local + "<" + remote }

// Compile checks remote name's config against meta and returns its mapping.
//
// Nil: rejected for meta.
func Compile(name string, p Remote, meta *metamodel.Metamodel) (*Mapping, error) {
	if meta == nil {
		return nil, fmt.Errorf("remotes: %q: no schema to check it against", name)
	}
	c := compiler{name: name, meta: meta, m: &Mapping{
		Name: name, Remote: p,
		Entities:  map[string]*EntityMapping{},
		Relations: map[string]*RelationMapping{},
		eval:      predicatefns.NewEvaluator(meta),
	}}
	if !principal.ValidIntegrationName(name) {
		c.fail("invalid name: use 1-63 lowercase letters, digits, '_' or '-', starting with a letter or digit")
	}
	c.remote(p)
	if len(c.errs) > 0 {
		return nil, errors.Join(c.errs...)
	}
	c.m.Hash = mappingHash(c.m)
	return c.m, nil
}

type compiler struct {
	name string
	meta *metamodel.Metamodel
	m    *Mapping
	errs []error
}

func (c *compiler) fail(format string, args ...any) {
	c.errs = append(c.errs, fmt.Errorf("remotes: %q: "+format, append([]any{c.name}, args...)...))
}

func (c *compiler) remote(p Remote) {
	if err := checkURL(p.URL, p.AllowInsecureHTTP); err != nil {
		c.fail("url: %v", err)
	}
	if p.Credential.IsZero() {
		c.fail("credential: name one source: env, file or secret")
	} else if err := p.Credential.ValidateDeclared(); err != nil {
		c.fail("credential: %v", err)
	}
	if p.AuthHeader != "" {
		if _, err := httpremote.Header(p.AuthHeader, "x"); err != nil {
			c.fail("auth_header: %v", err)
		}
	}
	if p.RunAs != "" {
		// Any `system:` name, as for a scheduled task's run_as.
		if err := principal.ValidateRunAs(p.RunAs, func(string) bool { return true }); err != nil {
			c.fail("%v", err)
		}
	}
	switch p.OnGone {
	case "", OnGoneDetach, OnGoneDelete:
	default:
		c.fail("on_gone %q must be %s or %s", p.OnGone, OnGoneDetach, OnGoneDelete)
	}
	if p.MaxGone != nil && *p.MaxGone < 0 {
		c.fail("max_gone must not be negative")
	}
	if p.MaxRows < 0 || p.MaxRows > MaxMaxRows {
		c.fail("max_rows %d must be between 0 (the default, %d) and %d", p.MaxRows, DefaultMaxRows, MaxMaxRows)
	}
	if p.MaxFetch < 0 || p.MaxFetch > MaxMaxFetch {
		c.fail("max_fetch %d must be between 0 (the default, %d) and %d", p.MaxFetch, DefaultMaxFetch, MaxMaxFetch)
	}
	if p.MaxBytes < 0 || p.MaxBytes > MaxMaxBytes {
		c.fail("max_bytes %d must be between 0 (the default, %d) and %d", p.MaxBytes, DefaultMaxBytes, MaxMaxBytes)
	}
	c.rule("conflict", p.Conflict)
	c.clockSkew(p.MaxClockSkew)
	if _, err := parseRetention(p.SupersededRetention); err != nil {
		c.fail("%v", err)
	}
	if len(p.Entities) == 0 {
		c.fail("entities: map at least one source type")
	}
	for _, src := range sortedKeys(p.Entities) {
		c.entity(src, p.Entities[src])
	}
	for _, src := range sortedKeys(p.Relations) {
		c.relation(src, p.Relations[src])
	}
}

func (c *compiler) entity(src string, em EntityMap) {
	local := em.Type
	if local == "" {
		local = src
	}
	if !validTypeName(src) {
		c.fail("entities: source type %q is not a valid type name", src)
		return
	}
	for _, other := range sortedKeys(c.m.Entities) {
		if c.m.Entities[other].Local == local {
			c.fail("entities: %q and %q both map to local type %q", other, src, local)
			return
		}
	}
	def, ok := c.meta.GetEntityDef(local)
	switch {
	case !ok:
		c.fail("entities: %q: unknown local type %q", src, local)
		return
	case len(def.Faces) > 0:
		c.fail("entities: %q: local type %q declares faces, which a pull does not support", src, local)
		return
	case def.IsManualID():
		c.fail("entities: %q: local type %q uses manual ids; a pull generates ids", src, local)
		return
	}
	refDef, ok := def.Properties[em.Ref]
	switch {
	case em.Ref == "":
		c.fail("entities: %q: ref names no property", src)
		return
	case !ok || refDef.Type != metamodel.PropertyTypeExternalRef:
		c.fail("entities: %q: ref %q is not an external_ref property of %q", src, em.Ref, local)
		return
	case refDef.Sync:
		c.fail("entities: %q: ref %q is declared sync: true, which is for Lua sync connectors", src, em.Ref)
		return
	}
	if c.m.System == "" {
		c.m.System = refDef.System
	} else if c.m.System != refDef.System {
		c.fail("entities: %q: ref %q has system %q, but the remote's other refs have %q",
			src, em.Ref, refDef.System, c.m.System)
		return
	}
	fields, err := typedFields(em.Fields, func(names []string) ([]syncmerge.Field, error) {
		return syncmerge.FieldsFor(c.meta, local, names)
	})
	if err != nil {
		c.fail("entities: %q: %v", src, err)
		return
	}
	for _, f := range fields {
		if f.Local.Name == em.Ref || f.Local.Kind == syncmerge.KindExternalRef {
			c.fail("entities: %q: field %q is an external ref; only the pull writes the ref", src, f.Local.Name)
			return
		}
	}
	c.rules(fmt.Sprintf("entities: %q", src), em.Conflict, em.FieldConflicts, fields)
	share, ok := c.share(src, local, em.Share)
	if !ok {
		return
	}
	c.m.Entities[src] = &EntityMapping{Source: src, Local: local, Ref: em.Ref, Fields: fields, Share: share}
}

// rule checks a conflict rule.
func (c *compiler) rule(where, rule string) {
	switch rule {
	case "", ConflictHold, ConflictLatest, ConflictLocal, ConflictRemote:
	default:
		c.fail("%s: unknown rule %q; use %s, %s, %s or %s",
			where, rule, ConflictHold, ConflictLatest, ConflictLocal, ConflictRemote)
	}
}

// rules checks a type's conflict rule and its per-field rules, and sets each
// field's rule: the field's own, else the type's, else the remote's, else
// hold.
func (c *compiler) rules(where, typeRule string, perField map[string]string, fields []FieldMapping) {
	c.rule(where+": conflict", typeRule)
	for _, field := range sortedKeys(perField) {
		if !slices.ContainsFunc(fields, func(f FieldMapping) bool { return f.Local.Name == field }) {
			c.fail("%s: field_conflicts: %q is not a mapped field", where, field)
			continue
		}
		c.rule(fmt.Sprintf("%s: field_conflicts: %q", where, field), perField[field])
	}
	for i := range fields {
		fields[i].Rule = cmp.Or(perField[fields[i].Local.Name], typeRule, c.m.Remote.Conflict, ConflictHold)
	}
}

// clockSkew checks max_clock_skew.
func (c *compiler) clockSkew(raw string) {
	if raw == "" {
		return
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 || d > MaxMaxClockSkew {
		c.fail("max_clock_skew %q must be a duration above 0 and at most %s", raw, MaxMaxClockSkew)
	}
}

// share compiles a type's share condition. It reads the local row only: a
// condition that follows relations is refused, because the push evaluates
// it per row from the row alone.
func (c *compiler) share(src, local, cond string) (*predicate.Program, bool) {
	if strings.TrimSpace(cond) == "" {
		return nil, true
	}
	if !c.m.Remote.Push {
		c.fail("entities: %q: share needs push: true", src)
		return nil, false
	}
	prog, err := c.m.eval.Compile(local, cond)
	if err != nil {
		c.fail("entities: %q: share: %v", src, err)
		return nil, false
	}
	if len(prog.Traversals()) > 0 {
		c.fail("entities: %q: share: related(...) is not supported; the condition reads the row only", src)
		return nil, false
	}
	return prog, true
}

func (c *compiler) relation(src string, rm RelationMap) {
	local := rm.Type
	if local == "" {
		local = src
	}
	if !validTypeName(src) {
		c.fail("relations: source type %q is not a valid type name", src)
		return
	}
	def, ok := c.meta.GetRelationDef(local)
	if !ok {
		c.fail("relations: %q: unknown local relation type %q", src, local)
		return
	}
	var locals []string
	for _, em := range c.m.Entities {
		locals = append(locals, em.Local)
	}
	if !slices.ContainsFunc(locals, func(t string) bool { return slices.Contains(def.From, t) }) ||
		!slices.ContainsFunc(locals, func(t string) bool { return slices.Contains(def.To, t) }) {

		c.fail("relations: %q: local type %q does not connect two mapped entity types", src, local)
		return
	}
	fields, err := typedFields(rm.Fields, func(names []string) ([]syncmerge.Field, error) {
		return syncmerge.FieldsForRelation(c.meta, local, names)
	})
	if err != nil {
		c.fail("relations: %q: %v", src, err)
		return
	}
	for _, f := range fields {
		if f.Local.Kind == syncmerge.KindExternalRef {
			c.fail("relations: %q: field %q is an external ref", src, f.Local.Name)
			return
		}
	}
	// A push finds the source type of a local relation by its local type, so
	// two source types may not share one.
	if c.m.Remote.Push {
		for _, other := range sortedKeys(c.m.Relations) {
			if c.m.Relations[other].Local == local {
				c.fail("relations: %q and %q both map to local type %q, which a remote with push does not support",
					other, src, local)
				return
			}
		}
	}
	if !c.relationRules(src, &rm, fields) {
		return
	}
	c.rules(fmt.Sprintf("relations: %q", src), rm.Conflict, rm.FieldConflicts, fields)
	c.m.Relations[src] = &RelationMapping{Source: src, Local: local, Fields: fields}
}

// relationRules refuses relation conflict rules that would never apply: a
// remote without push keeps the source's meta, and relation content is only
// pulled.
func (c *compiler) relationRules(src string, rm *RelationMap, fields []FieldMapping) bool {
	if rm.Conflict == "" && len(rm.FieldConflicts) == 0 {
		return true
	}
	if !c.m.Remote.Push {
		c.fail("relations: %q: conflict rules need push: true; a remote without push keeps the source's meta", src)
		return false
	}
	for _, f := range fields {
		if f.Local.Kind == syncmerge.KindContent && rm.FieldConflicts[f.Local.Name] != "" {
			c.fail("relations: %q: field_conflicts: %q is relation content, which is only pulled", src, f.Local.Name)
			return false
		}
	}
	return true
}

// typedFields types a local-to-source field map and checks the source
// names.
func typedFields(
	m map[string]string, typeOf func([]string) ([]syncmerge.Field, error),
) ([]FieldMapping, error) {
	names := sortedKeys(m)
	typed, err := typeOf(names)
	if err != nil {
		return nil, err
	}
	out := make([]FieldMapping, len(names))
	seen := map[string]string{}
	for i, name := range names {
		remote := m[name]
		if !validTypeName(remote) {
			return nil, fmt.Errorf("field %q: source field %q is not a valid name", name, remote)
		}
		if other, dup := seen[remote]; dup {
			return nil, fmt.Errorf("fields %q and %q both read source field %q", other, name, remote)
		}
		seen[remote] = name
		out[i] = FieldMapping{Local: typed[i], Remote: remote}
	}
	return out, nil
}

// checkURL allows what httpremote accepts: an absolute http or https URL
// with a host and no credentials, query or fragment, and plain http only to
// a loopback host unless allowInsecure.
func checkURL(raw string, allowInsecure bool) error {
	if raw == "" {
		return errors.New("required")
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return errors.New("does not parse")
	case u.Scheme != "http" && u.Scheme != "https":
		return errors.New("must start with https:// or http://")
	case u.Hostname() == "":
		return errors.New("has no host")
	case u.User != nil:
		return errors.New("must not hold credentials; use credential")
	case u.RawQuery != "" || u.Fragment != "" || u.ForceQuery:
		return errors.New("must not have a query or fragment")
	}
	if u.Scheme == "http" && !allowInsecure && !loopback(u.Hostname()) {
		return errors.New("plain http to a host that is not loopback needs allow_insecure_http")
	}
	return nil
}

func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// validTypeName is the grammar a source type or field name, and a plural
// the source serves, must match before it becomes part of a path.
func validTypeName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case (c == '_' || c == '-') && i > 0:
		default:
			return false
		}
	}
	return true
}

// mappingHash is a digest of everything about m that changes how a source
// row maps onto a local row.
func mappingHash(m *Mapping) string {
	type field struct{ Local, Remote string }
	type typ struct {
		Source, Local, Ref string
		Fields             []field
	}
	var v struct {
		System              string
		Entities, Relations []typ
	}
	v.System = m.System
	conv := func(fs []FieldMapping) []field {
		out := make([]field, len(fs))
		for i, f := range fs {
			out[i] = field{f.Local.Name, f.Remote}
		}
		return out
	}
	for _, src := range sortedKeys(m.Entities) {
		e := m.Entities[src]
		v.Entities = append(v.Entities, typ{e.Source, e.Local, e.Ref, conv(e.Fields)})
	}
	for _, src := range sortedKeys(m.Relations) {
		r := m.Relations[src]
		v.Relations = append(v.Relations, typ{Source: r.Source, Local: r.Local, Fields: conv(r.Fields)})
	}
	b, err := json.Marshal(v)
	if err != nil { // coverage-ignore: invariant: strings and slices always marshal
		panic(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (p Remote) onGone() string {
	if p.OnGone == "" {
		return OnGoneDetach
	}
	return p.OnGone
}

func (p Remote) maxGone() int {
	if p.MaxGone == nil {
		return DefaultMaxGone
	}
	return *p.MaxGone
}

func (p Remote) maxRows() int {
	if p.MaxRows == 0 {
		return DefaultMaxRows
	}
	return p.MaxRows
}

func (p Remote) maxFetch() int {
	if p.MaxFetch == 0 {
		return DefaultMaxFetch
	}
	return p.MaxFetch
}

// maxClockSkew is the parsed max_clock_skew. [Compile] has checked it.
func (p Remote) maxClockSkew() time.Duration {
	d, err := time.ParseDuration(p.MaxClockSkew)
	if err != nil || d <= 0 {
		return DefaultMaxClockSkew
	}
	return d
}

// supersededRetention is the parsed superseded_retention. [Compile] has
// checked it.
func (p Remote) supersededRetention() time.Duration {
	d, err := parseRetention(p.SupersededRetention)
	if err != nil {
		return DefaultSupersededRetention
	}
	return d
}

// maxRetentionDays bounds a retention in days, so it cannot overflow.
const maxRetentionDays = 36500

// parseRetention parses a superseded_retention value: empty for the
// default, a whole number of days ("30d"), or a Go duration ("12h", "0").
func parseRetention(raw string) (time.Duration, error) {
	if raw == "" {
		return DefaultSupersededRetention, nil
	}
	d, err := time.ParseDuration(raw)
	if days, ok := strings.CutSuffix(raw, "d"); ok {
		n, aerr := strconv.Atoi(days)
		if aerr == nil && n > maxRetentionDays {
			aerr = errors.New("too long")
		}
		d, err = time.Duration(n)*24*time.Hour, aerr
	}
	if err != nil || d < 0 {
		return 0, fmt.Errorf("superseded_retention %q must be a duration of 0 or more, such as 30d or 12h", raw)
	}
	return d, nil
}

func (p Remote) maxBytes() int64 {
	if p.MaxBytes == 0 {
		return DefaultMaxBytes
	}
	return p.MaxBytes
}

// Settings are a remote's settings with their defaults applied, as a
// status page shows them. They hold no credential: only the kind of source
// it is read from.
type Settings struct {
	URL                 string
	Description         string
	CredentialKind      string
	RunAs               string
	Every               string
	Push                bool
	Conflict            string
	OnGone              string
	MaxClockSkew        time.Duration
	SupersededRetention time.Duration
}

// Settings returns p's settings with their defaults applied.
func (p Remote) Settings() Settings {
	conflict := p.Conflict
	if conflict == "" {
		conflict = ConflictHold
	}
	return Settings{
		URL: p.URL, Description: p.Description, CredentialKind: p.Credential.Kind(), RunAs: p.RunAs,
		Every: p.Every, Push: p.Push, Conflict: conflict, OnGone: p.onGone(),
		MaxClockSkew: p.maxClockSkew(), SupersededRetention: p.supersededRetention(),
	}
}
