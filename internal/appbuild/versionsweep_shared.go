//go:build postgres || sqlite

package appbuild

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The backend-neutral half of the version wiring, shared by every build that
// has a versioning store.
//
// It lives in its own file rather than in either recipe because none of it is
// backend-specific: the projection comes from the metamodel, the cadence from
// the environment, and the version capability is named in store-package terms.
// Duplicating it per backend is how the recipes drift — the same reason
// prepare/assemble exist.

// metaProjectionProvider yields the render-schema projection of one metamodel,
// for a version sweep to stamp on create/update versions. It is bound to the
// metamodel its assembly loaded: a reload assembles again, and that assembly
// builds a new sweep with the new metamodel.
type metaProjectionProvider struct {
	meta *metamodel.Metamodel
}

func (p metaProjectionProvider) Projection() (hash string, projectionJSON []byte) {
	proj := metamodel.NewRenderProjection(p.meta)
	b, err := proj.JSON()
	if err != nil {
		// Unreachable short of a runtime bug (RenderProjection is trivially
		// marshalable). Return an empty hash so the sweep tick skips capture
		// this round rather than stamping an empty projection.
		return "", nil
	}
	return proj.Hash(), b
}

// versionServiceProvider is the capability versionServiceFor needs: a store that
// can hand out a versioning service (history reads, version writes, purge)
// sharing its own connection.
//
// Returns the store.VersionService interface, so a backend satisfies this
// without importing pgstore (TKT-L3FNEN). An ALIAS for store.VersionServiceProvider,
// not a locally-declared interface: the consumer-side rule is about owning the
// minimum method list, and at one method there is nothing to narrow.
type versionServiceProvider = store.VersionServiceProvider

// newVersionSweep builds, but does not start, the reconciliation sweep for st:
// the backend constructor (newBackendVersionSweep, one per build tag) fed the
// metamodel projection and the cadence from sweepConfigFromEnv, so a test or
// dev deployment can make create/update versions appear quickly (production
// uses the zero-value defaults: 5m interval/idle).
//
// Returns a genuinely nil sweep and no error for a store this build has no
// sweep for, such as the memstore the shared tests assemble over.
//
// Nil: meta is rejected with an error when st has a sweep, because every tick
// stamps the metamodel projection.
func newVersionSweep(st store.Store, meta *metamodel.Metamodel) (versionSweep, error) {
	var provider store.ProjectionProvider
	if meta != nil {
		provider = metaProjectionProvider{meta: meta}
	}
	sw, err := newBackendVersionSweep(st, provider, sweepConfigFromEnv())
	if err != nil {
		return nil, fmt.Errorf("appbuild: build version sweep: %w", err)
	}
	return sw, nil
}

// versionServiceFor returns the store's versioning service (history reads,
// version writes, purge) sharing its connection. Returns a genuinely nil
// interface — never a typed nil — both for a store without the capability and
// for one whose handle came back nil, so nil-checks downstream behave
// correctly.
//
// The guard goes through nonNilCapability rather than a bare `== nil`, and that
// distinction is the whole point: VersionStore() returns an INTERFACE, so a
// backend returning a nil pointer yields a non-nil interface that a plain check
// cannot see. versionRecorderFor (appbuild.go) and startDataMigration would
// both pass their nil-checks and panic on first use — at write time, in
// production. See capability.go.
func versionServiceFor(st store.Store) store.VersionService {
	s, ok := st.(versionServiceProvider)
	if !ok {
		return nil
	}
	return nonNilCapability(s.VersionStore())
}

// versionTaggerFor returns the store's version tagger (TKT-VO6VG9), built
// with the same metamodel projection the sweep stamps, so a version that
// tagging the current state captures is the one the sweep would have written.
// Returns a genuinely nil interface and no error for a store without the
// capability, which the caller treats as "version tags are not available".
// A backend that has the capability but refuses to build a tagger is an
// error: reporting it as an unsupported backend would hide a wiring fault.
//
// Nil: meta is rejected with an error, because a capture without a
// projection could not be rendered.
func versionTaggerFor(st store.Store, meta *metamodel.Metamodel) (store.VersionTagger, error) {
	p, ok := versionServiceFor(st).(store.VersionTaggerProvider)
	if !ok {
		return nil, nil //nolint:nilnil // no history means no tagger, which is not an error
	}
	if meta == nil {
		return nil, errors.New("appbuild: version tags need the metamodel")
	}
	tg, err := p.VersionTagger(metaProjectionProvider{meta: meta})
	if err != nil {
		return nil, fmt.Errorf("appbuild: build version tagger: %w", err)
	}
	return nonNilCapability(tg), nil
}

// versionTagReaderFor returns the tag reads the script read surfaces serve
// rela.version_by_tag and rela.tagged_entities from: the store's version
// service, which answers both without a projection. Nil, as a genuinely nil
// interface, when st keeps no history or no tags.
func versionTagReaderFor(st store.Store) visibility.VersionTagReader {
	l, ok := versionServiceFor(st).(visibility.VersionTagReader)
	if !ok {
		return nil
	}
	return nonNilCapability(l)
}

// sweepConfigFromEnv reads optional sweep-cadence overrides from the environment.
// All zero by default (→ the backend's 5m/5m/1h/500 defaults). Intended for
// e2e/dev where waiting minutes for create/update capture is impractical:
//
//	RELA_VERSION_SWEEP_INTERVAL / _IDLE / _MAX_STALENESS  (Go durations, e.g. 500ms)
//
// Unparseable values are ignored with a warning rather than failing boot — a
// misconfigured cadence must never take down the server; it just falls back to
// the default for that field.
func sweepConfigFromEnv() store.SweepConfig {
	dur := func(env string) time.Duration {
		v := os.Getenv(env)
		if v == "" {
			return 0
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			// G706 flags v as tainted because its analysis stops at the slog
			// call and cannot see the handler that encodes the record. The
			// message is a constant and v travels as a structured ATTRIBUTE,
			// which slog's handlers quote and escape — so an injected newline
			// cannot forge a second record. That is the same invariant
			// internal/dataentry pins in TestSlogTextHandlerEscapesNewlines.
			//
			// The value is worth echoing: this is an operator debugging their
			// own typo'd RELA_VERSION_SWEEP_* setting, and a warning that
			// withholds the rejected value is much harder to act on.
			//nolint:gosec // G706: constant message, user data as an escaped attribute
			slog.Warn("appbuild: ignoring invalid sweep duration",
				"env", env, "value", v, "error", err)
			return 0
		}
		return d
	}
	return store.SweepConfig{
		Interval:     dur("RELA_VERSION_SWEEP_INTERVAL"),
		Idle:         dur("RELA_VERSION_SWEEP_IDLE"),
		MaxStaleness: dur("RELA_VERSION_SWEEP_MAX_STALENESS"),
	}
}
