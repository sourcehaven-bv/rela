package attachment

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// recordingProcessor captures what it was asked to process and can rewrite the
// stream, change the file name, or reject.
type recordingProcessor struct {
	needsFull bool
	gotCtx    ProcessContext
	gotBytes  string
	replace   string // when non-empty, replaces the output bytes
	rename    string // when non-empty, ProcessInfo.FileName
	rejectMsg string // when non-empty, reject with this message
}

func (p *recordingProcessor) NeedsFullFile() bool { return p.needsFull }

func (p *recordingProcessor) Process(
	_ context.Context, pc ProcessContext, r io.Reader,
) (io.Reader, ProcessInfo, error) {
	p.gotCtx = pc
	b, _ := io.ReadAll(r)
	p.gotBytes = string(b)
	if p.rejectMsg != "" {
		return nil, ProcessInfo{}, Rejectedf("%s", p.rejectMsg)
	}
	out := string(b)
	if p.replace != "" {
		out = p.replace
	}
	return strings.NewReader(out), ProcessInfo{FileName: p.rename}, nil
}

func TestRunProcessor_NilIsNoopZeroCopy(t *testing.T) {
	in := strings.NewReader("hello")
	out, name, err := runProcessor(context.Background(), nil, ProcessContext{FileName: "f.txt"}, in, 1<<20)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "f.txt" {
		t.Errorf("name = %q, want f.txt", name)
	}
	// The no-op must thread the *same* reader through (zero-copy): identity check.
	if out != in {
		t.Errorf("no-op processor did not pass the original reader through")
	}
}

func TestRunProcessor_BuffersAndRewrites(t *testing.T) {
	p := &recordingProcessor{needsFull: true, replace: "CLEAN", rename: "f.jpg"}
	pc := ProcessContext{EntityID: "E1", EntityType: "doc", Property: "att", FileName: "f.heic"}
	out, name, err := runProcessor(context.Background(), p, pc, strings.NewReader("RAW"), 1<<20)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if p.gotBytes != "RAW" {
		t.Errorf("processor saw %q, want RAW", p.gotBytes)
	}
	if p.gotCtx != pc {
		t.Errorf("processor ctx = %+v, want %+v", p.gotCtx, pc)
	}
	if name != "f.jpg" {
		t.Errorf("name = %q, want f.jpg (rename applied)", name)
	}
	b, _ := io.ReadAll(out)
	if string(b) != "CLEAN" {
		t.Errorf("output = %q, want CLEAN (rewrite applied)", string(b))
	}
}

func TestRunProcessor_RejectionWrapsErrRejected(t *testing.T) {
	p := &recordingProcessor{needsFull: true, rejectMsg: "virus found"}
	_, _, err := runProcessor(context.Background(), p, ProcessContext{FileName: "x"}, strings.NewReader("EICAR"), 1<<20)
	if err == nil {
		t.Fatal("expected rejection error")
	}
	if !errors.Is(err, ErrRejected) {
		t.Errorf("error %v does not wrap ErrRejected", err)
	}
	if !strings.Contains(err.Error(), "virus found") {
		t.Errorf("error %q missing message", err.Error())
	}
}

func TestRunProcessor_EnforcesSizeCap(t *testing.T) {
	p := &recordingProcessor{needsFull: true}
	// 100 bytes through a 10-byte cap must fail before Process is reached.
	_, _, err := runProcessor(context.Background(), p, ProcessContext{FileName: "x"}, strings.NewReader(strings.Repeat("a", 100)), 10)
	if err == nil {
		t.Fatal("expected size-cap error")
	}
	if p.gotBytes != "" {
		t.Errorf("processor should not have run on over-cap input")
	}
}

func TestNoopProcessor_PassThrough(t *testing.T) {
	noop := NoopProcessor{}
	if noop.NeedsFullFile() {
		t.Error("NoopProcessor.NeedsFullFile should be false")
	}
	in := strings.NewReader("data")
	out, info, err := noop.Process(context.Background(), ProcessContext{}, in)
	if err != nil || out != in || info.FileName != "" {
		t.Errorf("no-op altered the stream: out==in? %v info=%+v err=%v", out == in, info, err)
	}
}
