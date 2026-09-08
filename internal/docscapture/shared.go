package docscapture

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// SharedProject is ONE stood-up temp project shared by every island of ONE
// document: the api{} client and the screenshot{}/page{} capturer both reach
// the same store, the same server, and the same scratch backend.
//
// # Why the two used to be separate, and why that was wrong
//
// api{} and screenshot{} each called standUp independently, so a manual had
// two temp projects with two stores. A write made by an api{} island was
// therefore INVISIBLE to every later figure. That is not a subtle limitation:
// it made the interesting figure of the worlds manual impossible to take. The
// manual can assert that publishing is a copy (an api{} POST), and can then
// only DESCRIBE the consequence in prose, because a screenshot of the
// published face's history would have been taken against a store where the
// copy never happened.
//
// Sharing one project makes the manual a single narrative: islands run
// top-to-bottom, and what an earlier one wrote a later one can photograph.
//
// # Scope is the DOCUMENT
//
// One project per manual, torn down when that manual finishes. Not per
// process: two manuals built in the same run must not see each other's writes,
// or a fixture from one would appear in the other's figures — and the failure
// would depend on build order, which is the worst kind. Not per island either:
// that is what this type exists to stop.
//
// On the postgres build the scratch schema follows the same scope, so a build
// creates ONE schema rather than one per consumer, and drops it CASCADE at
// teardown — including on the error path, since Close is deferred by the
// caller that owns the document.
//
// # Concurrency
//
// The mutex serializes stand-up and seeding. Islands execute sequentially, so
// it is not contended in practice; it is here because Capturer and APIClient
// are distinct objects reaching one holder, and "the doc runtime is
// single-threaded" is not a property this type can see.
//
// Nil: a nil *SharedProject is not usable — use NewSharedProject.
type SharedProject struct {
	projectDir string

	mu sync.Mutex
	// proj is the stood-up project, created on first use by whichever consumer
	// reaches it first.
	proj *project
	// spaChecked records that the embedded SPA precondition has been verified,
	// so the check runs once rather than per capture.
	spaChecked bool
	closed     bool
}

// NewSharedProject returns a holder for one document's temp project. Nothing is
// stood up yet: a manual with neither api{} nor screenshot{} pays nothing.
func NewSharedProject(projectDir string) *SharedProject {
	return &SharedProject{projectDir: projectDir}
}

// acquire returns the shared project, standing it up on first use and applying
// any seed ops beyond those already applied.
//
// # The seed applies ONCE and accumulates
//
// The seed is applied at stand-up, and each later call applies only the ops
// beyond the watermark (see project.syncSeed). Nothing is replayed and nothing
// is re-seeded, which is what lets a real write — an `edit()` through the
// entitymanager, or an api{} POST — survive into every later island rather
// than being clobbered by a re-application of the fixture.
//
// needSPA is checked at the point of FIRST SCREENSHOT rather than at stand-up,
// because the project may well have been stood up by an api{} island first,
// and api{} legitimately needs no built frontend.
func (s *SharedProject) acquire(
	ctx context.Context, projectDir string, seed []docs.SeedOp, needSPA bool,
) (*project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("the document's temp project is closed")
	}

	if needSPA && !s.spaChecked {
		if err := dataentry.CheckEmbeddedSPA(); err != nil {
			return nil, fmt.Errorf("data-entry SPA not built (run `just build-frontend`): %w", err)
		}
		s.spaChecked = true
	}

	if s.proj == nil {
		dir := projectDir
		if dir == "" {
			dir = s.projectDir
		}
		if dir == "" {
			return nil, errors.New("no project directory to serve (build with --project)")
		}
		// needSPA is already verified above; standUp re-checks harmlessly.
		p, err := standUp(ctx, dir, seed, needSPA)
		if err != nil {
			return nil, err
		}
		s.proj = p
		return p, nil
	}

	if err := s.proj.syncSeed(ctx, seed); err != nil {
		return nil, fmt.Errorf("seeding: %w", err)
	}
	return s.proj, nil
}

// Close tears down the document's project: server, services, scratch backend
// (DROP SCHEMA CASCADE on postgres), then the temp directory.
//
// Idempotent, and safe to defer on the error path — which is where it matters
// most, since a failed build must not leave a scratch schema behind.
func (s *SharedProject) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.proj != nil {
		s.proj.close()
		s.proj = nil
	}
	return nil
}
