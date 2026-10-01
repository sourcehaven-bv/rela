package lua

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewReader_RefusesUnsetWorld pins RR-HKVULG: a runtime built with no
// world would fail every list read at run time, far from the wiring site
// that forgot it, so construction panics instead.
func TestNewReader_RefusesUnsetWorld(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewReader with an unset world did not panic")
		}
		if msg, _ := r.(string); !strings.Contains(msg, "ReadDeps.World is unset") {
			t.Errorf("panic = %v, want it to name ReadDeps.World", r)
		}
	}()
	NewReader(ReadDeps{}, &bytes.Buffer{})
}

// TestNewDetached_RunsWithoutGraph pins that a detached runtime, built with
// no graph collaborators, passes the world check and runs a script.
func TestNewDetached_RunsWithoutGraph(t *testing.T) {
	var out bytes.Buffer
	rt := NewDetached(&out)
	defer rt.Close()
	if err := rt.RunString(`rela.output("sent")`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "sent") {
		t.Errorf("output = %q, want it to contain %q", got, "sent")
	}
}
