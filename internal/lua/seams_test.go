package lua

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type seamRoundTrip func(*http.Request) (*http.Response, error)

func (f seamRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// countingClient answers every request with 200 and counts them.
func countingClient(n *atomic.Int32) *http.Client {
	return &http.Client{Transport: seamRoundTrip(func(r *http.Request) (*http.Response, error) {
		n.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})}
}

type seamAI struct{ answer string }

func (p seamAI) Chat(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return &ai.ChatResponse{Content: p.answer}, nil
}

func (seamAI) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, errors.New("unused")
}

// TestSeams_WinOverOptions pins that a seam on the deps beats the matching
// per-call option, for each seam.
func TestSeams_WinOverOptions(t *testing.T) {
	seamDay := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	optionDay := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var seamHits, optionHits atomic.Int32
	seamOut, optionOut := t.TempDir(), t.TempDir()

	ws := newMockWorkspace(t)
	deps := ws.services(t.TempDir())
	deps.Seams = Seams{
		Clock:      func() time.Time { return seamDay },
		HTTPClient: countingClient(&seamHits),
		AI:         seamAI{answer: "seam"},
		OutputDir:  seamOut,
	}
	var buf bytes.Buffer
	rt := NewWriter(deps, &buf,
		WithClock(func() time.Time { return optionDay }),
		WithHTTPClient(countingClient(&optionHits)),
		WithAIProvider(seamAI{answer: "option"}),
		WithOutputDir(optionOut),
		WithCapabilities(Capabilities{HTTP: true, AI: true, WriteFile: true}))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
		assert(rela.today == "2020-01-01", "today = " .. rela.today)
		local _, herr = http.get("http://example.invalid/")
		assert(herr == nil, "http: " .. tostring(herr and herr.message))
		local r, aerr = ai.chat({messages = {{role = "user", content = "hi"}}})
		assert(aerr == nil and r.content == "seam", "ai answered " .. tostring(r and r.content))
		rela.write_file("x.txt", "x")
	`))
	require.EqualValues(t, 1, seamHits.Load())
	require.Zero(t, optionHits.Load())
	_, err := os.Stat(filepath.Join(seamOut, "x.txt"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(optionOut, "x.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestWithHTTPClient_PerRuntime pins that two runtimes keep their own
// clients rather than sharing the process-wide one.
func TestWithHTTPClient_PerRuntime(t *testing.T) {
	var a, b atomic.Int32
	run := func(n *atomic.Int32) {
		ws := newMockWorkspace(t)
		var buf bytes.Buffer
		rt := NewWriter(ws.services(t.TempDir()), &buf, WithHTTPClient(countingClient(n)),
			WithCapabilities(Capabilities{HTTP: true}))
		defer rt.Close()
		require.NoError(t, rt.RunString(`local _, err = http.get("http://example.invalid/"); assert(err == nil)`))
	}
	run(&a)
	run(&a)
	run(&b)
	require.EqualValues(t, 2, a.Load())
	require.EqualValues(t, 1, b.Load())
}

// TestSeams_AIOnlyOnWriters pins that a reader never gets the AI seam, as it
// never gets the ai.yaml provider.
func TestSeams_AIOnlyOnWriters(t *testing.T) {
	var buf bytes.Buffer
	rt := NewReader(ReadDeps{World: store.TrivialScope(), Seams: Seams{AI: seamAI{answer: "seam"}}}, &buf,
		WithCapabilities(Capabilities{AI: true}))
	t.Cleanup(rt.Close)
	require.Nil(t, rt.egress.ai)
}
