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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
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
	// source owns these fields; every other local field is local.
	Fields map[string]string `yaml:"fields,omitempty"`
}

// RelationMap maps one source relation type.
type RelationMap struct {
	// Type is the local type; empty means the source type's name.
	Type string `yaml:"type,omitempty"`
	// Fields maps local relation meta to source meta; "content" is the
	// body. The source wins on these.
	Fields map[string]string `yaml:"fields,omitempty"`
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
}

// EntityMapping maps one source entity type.
type EntityMapping struct {
	Source, Local, Ref string
	Fields             []FieldMapping
}

// RelationMapping maps one source relation type.
type RelationMapping struct {
	Source, Local string
	Fields        []FieldMapping
}

// FieldMapping is one owned field: the local field, typed, and the source
// field it is read from.
type FieldMapping struct {
	Local  syncmerge.Field
	Remote string
}

// baseKey names the field in a ledger row's base. It holds both names, so a
// field mapped to another source field starts without a base.
func (f FieldMapping) baseKey() string { return f.Local.Name + "<" + f.Remote }

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

// CredentialEnvPrefix starts the name of every environment variable a
// remote may read its credential from.
const CredentialEnvPrefix = "REMOTE_CREDENTIAL_" //nolint:gosec // G101: a variable name prefix, not a credential

// checkCredentialName refuses a credential source whose name does not mark
// it as a remote credential. remotes.yaml sends the value to the URL it
// names, so a source it may name freely would let whoever edits the file
// send any host secret off the machine: RELA_DATABASE_URL, the token key,
// or a file under $CREDENTIALS_DIRECTORY. An env name starts with
// [CredentialEnvPrefix]; a file is an absolute path whose base name is a
// remote_credential. name, as a secret is. A relative path is refused
// because it would resolve against whatever directory the process runs in.
func checkCredentialName(cr httpremote.Credential) error {
	switch {
	case cr.Env != "":
		if !strings.HasPrefix(cr.Env, CredentialEnvPrefix) || len(cr.Env) == len(CredentialEnvPrefix) {
			return fmt.Errorf("env %q: the variable name must start with %q followed by a name",
				cr.Env, CredentialEnvPrefix)
		}
	case cr.File != "":
		if !filepath.IsAbs(cr.File) {
			return fmt.Errorf("file %q: the path must be absolute", cr.File)
		}
		if !secrets.IsRemoteCredential(filepath.Base(cr.File)) {
			return fmt.Errorf("file %q: the file name must start with %q followed by a name",
				cr.File, secrets.RemoteCredentialPrefix)
		}
	}
	return nil
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
	} else if err := p.Credential.Validate(); err != nil {
		c.fail("credential: %v", err)
	} else if err := checkCredentialName(p.Credential); err != nil {
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
	c.m.Entities[src] = &EntityMapping{Source: src, Local: local, Ref: em.Ref, Fields: fields}
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
	c.m.Relations[src] = &RelationMapping{Source: src, Local: local, Fields: fields}
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

func (p Remote) maxBytes() int64 {
	if p.MaxBytes == 0 {
		return DefaultMaxBytes
	}
	return p.MaxBytes
}
