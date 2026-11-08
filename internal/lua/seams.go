package lua

import (
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/ai"
)

// Seams replaces what a runtime would otherwise take from the host: the
// clock, outbound HTTP, the AI provider, the mail transport and the
// directory rela.write_file writes to. A wiring site sets them on
// [ReadDeps.Seams] once, so every runtime built from those deps gets them.
// appbuild's in-memory world uses this to keep a test run off the network
// and out of the project tree.
//
// A seam wins over the matching per-call [Option]. The seams are the world's
// guarantee, and a per-call option (a CLI --output flag, say) must not
// undo it.
//
// Only runtimes built from deps that carry the seams see them. A surface that
// builds its own [ReadDeps] (the data-entry app, the docs runtime) does not.
//
// The zero value replaces nothing.
type Seams struct {
	// Clock backs rela.today, rela.now and the date helpers.
	//
	// Nil: accepted — the [WithClock] option, else the wall clock.
	Clock func() time.Time

	// HTTPClient is the client the http.* bindings send through.
	//
	// Nil: accepted — the [WithHTTPClient] option, else the shared client.
	HTTPClient *http.Client

	// AI is the provider the ai.* bindings use on a writer runtime. It wins
	// over the provider ai.yaml configures; script.NewWriterRuntime then does
	// not read ai.yaml at all. A reader runtime never gets it, as it never
	// gets the ai.yaml provider.
	//
	// Nil: accepted — ai.yaml decides.
	AI ai.Provider

	// Mail builds the mail.send transport in place of the one mail.yaml
	// names. script.NewWriterRuntime passes it to [LoadContextOptions].
	//
	// Nil: accepted — mail.yaml decides.
	Mail MailSenderLoader

	// OutputDir is where rela.write_file writes. An absolute path is used as
	// is; a relative one is joined to the project root.
	//
	// Empty: the [WithOutputDir] option, else output/ under the root.
	OutputDir string
}

// applySeams sets the seams of s on r. newRuntime calls it after the
// options, so a seam wins over the matching option.
func applySeams(r *Runtime, s Seams, writer bool) {
	if s.Clock != nil {
		r.clock = s.Clock
	}
	if s.HTTPClient != nil {
		r.egress.http = s.HTTPClient
	}
	if s.AI != nil && writer {
		r.egress.ai = s.AI
	}
	if s.OutputDir != "" {
		r.outputDir = s.OutputDir
	}
}
