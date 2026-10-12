package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// configReader reads one project-root config file; config.Loader satisfies
// it. A missing file satisfies errors.Is(err, fs.ErrNotExist).
type configReader interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// RealtimeSnapshot is the realtime configuration one operation reads. Load
// it once per operation (CLAUDE.md "Capture state once per operation").
type RealtimeSnapshot struct {
	// Config is the compiled realtime.yaml; nil when the project has none,
	// which means realtime is off.
	Config *realtimecfg.Compiled
	// ReadOnly is set when a schema or acl.yaml reload made the file
	// invalid. Config is then the last valid one, kept so existing channels
	// stay readable, but it was checked against a schema and policy that no
	// longer hold, so everything that reads entity data must fail closed:
	// triggers and auto rules do not run, lifecycle operations with a
	// condition are denied, and a role sourced from an entity property
	// grants nobody.
	ReadOnly bool
	// Problem is why ReadOnly is set; nil otherwise.
	Problem error
}

// RealtimeConfig holds the compiled realtime.yaml for one assembled
// Services and reloads it. Readers take a [RealtimeSnapshot] through an
// atomic pointer and never block; Reload serializes writers with a separate
// mutex.
//
// Nil: accepted by every method — a Services built by NewFromCollaborators
// has no holder, and a nil holder is realtime off.
type RealtimeConfig struct {
	reader configReader
	meta   *metamodel.Metamodel
	policy func() (*acl.Policy, error)

	mu  sync.Mutex
	cur atomic.Pointer[RealtimeSnapshot]
	// last is the most recent non-nil config, guarded by mu. Reload checks
	// anchors against it so deleting realtime.yaml and adding it back with
	// another anchor cannot bypass [realtimecfg.CheckReload].
	last *realtimecfg.Compiled
}

// Load returns the current snapshot.
func (r *RealtimeConfig) Load() RealtimeSnapshot {
	if r == nil {
		return RealtimeSnapshot{}
	}
	return *r.cur.Load()
}

// Reload re-reads realtime.yaml against the schema and policy the holder
// was built with. A file that fails to load, or a change [realtimecfg.CheckReload]
// refuses, leaves the previous snapshot in place and returns the problem:
// the schema and acl.yaml did not change, so the previous config is still
// valid. A deleted file turns realtime off.
func (r *RealtimeConfig) Reload(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next, err := loadRealtime(ctx, r.reader, r.meta, r.policy)
	if err != nil {
		return err
	}
	if err := realtimecfg.CheckReload(r.last, next); err != nil {
		return err
	}
	if next != nil {
		r.last = next
	}
	r.cur.Store(&RealtimeSnapshot{Config: next})
	return nil
}

// Realtime returns the realtime configuration holder of svc. A package
// function because Services is at its plimsoll cap.
//
// Nil: svc is accepted and returns nil, as does a Services without a holder.
func Realtime(svc *Services) *RealtimeConfig {
	if svc == nil {
		return nil
	}
	return svc.realtime
}

// WithRealtimeFallback makes a realtime.yaml that no longer loads keep
// prev's configuration read-only (see [RealtimeSnapshot.ReadOnly]), instead
// of failing the build.
// It is for a successor built by a schema hot reload (MCP's watchSchema):
// the schema change is already on disk, so refusing it would leave the old
// schema serving with nothing to show for it, while running triggers
// checked against the old schema could post what the new acl.yaml hides.
//
// Nil: prev is accepted and means no fallback.
func WithRealtimeFallback(prev *Services) Option {
	return func(o *options) { o.realtimeFallback = prev }
}

// LoadRealtime reads realtime.yaml through r and compiles it against meta
// and policy. It returns (nil, nil) when the file is absent. `rela acl
// audit` and the Configure preview use it; the boot path goes through
// [NewSharedBase].
//
// Nil: meta is rejected; policy is accepted (no acl.yaml).
func LoadRealtime(ctx context.Context, r configReader, meta *metamodel.Metamodel,
	policy *acl.Policy,
) (*realtimecfg.Compiled, error) {
	return loadRealtime(ctx, r, meta, func() (*acl.Policy, error) { return policy, nil })
}

// loadRealtime resolves the policy only when the file exists, so a project
// without realtime.yaml never pays for (or fails on) a second acl.yaml read.
func loadRealtime(ctx context.Context, r configReader, meta *metamodel.Metamodel,
	policy func() (*acl.Policy, error),
) (*realtimecfg.Compiled, error) {
	data, err := r.Load(ctx, realtimecfg.FileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // (nil, nil) is "no realtime.yaml": realtime off
		}
		return nil, fmt.Errorf("read %s: %w", realtimecfg.FileName, err)
	}
	p, err := policy()
	if err != nil {
		return nil, err
	}
	return realtimecfg.Parse(data, meta, p)
}

// newRealtimeConfig loads realtime.yaml for a base. The policy is the one
// prepare loaded, else the injected Declarative's, else acl.yaml read again:
// an injected NopACL or ReadOnlyACL changes who may write, not which fields
// acl.yaml hides from readers, and the leak checks need the latter.
func newRealtimeConfig(cfg Config, o *options, loaded *acl.Policy,
	meta *metamodel.Metamodel,
) (*RealtimeConfig, error) {
	reader := cfg.configLoader()
	resolve := policyOnce(func() (*acl.Policy, error) {
		if loaded != nil || o.acl == nil {
			return loaded, nil
		}
		if d, ok := o.acl.(*acl.Declarative); ok {
			return d.Policy(), nil
		}
		return loadACLPolicy(cfg)
	})
	compiled, err := loadRealtime(context.Background(), reader, meta, resolve)
	r := &RealtimeConfig{reader: reader, meta: meta, policy: resolve, last: compiled}
	if err == nil {
		r.cur.Store(&RealtimeSnapshot{Config: compiled})
		return r, nil
	}
	prev := Realtime(o.realtimeFallback)
	if prev == nil {
		return nil, err
	}
	slog.Error("realtime.yaml no longer loads against the reloaded schema; "+
		"keeping the previous realtime config read-only", "error", err)
	prev.mu.Lock()
	r.last = prev.last
	prev.mu.Unlock()
	r.cur.Store(&RealtimeSnapshot{Config: prev.Load().Config, ReadOnly: true, Problem: err})
	return r, nil
}

// policyOnce memoizes the first successful policy read. Unlike
// [sync.OnceValues] it does not cache an error, so a Reload after a
// transient acl.yaml read failure tries again.
func policyOnce(read func() (*acl.Policy, error)) func() (*acl.Policy, error) {
	var (
		mu   sync.Mutex
		done bool
		got  *acl.Policy
	)
	return func() (*acl.Policy, error) {
		mu.Lock()
		defer mu.Unlock()
		if done {
			return got, nil
		}
		p, err := read()
		if err != nil {
			return nil, err
		}
		got, done = p, true
		return got, nil
	}
}
