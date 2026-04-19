package encryption

import (
	"errors"
	"strings"
	"testing"
)

func TestMustStdlibContract_PassesThrough(t *testing.T) {
	got := mustStdlibContract(42, nil)
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestMustStdlibContract_PanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not string: %T", r)
		}
		if !strings.Contains(msg, "stdlib contract broken") {
			t.Fatalf("panic msg missing prefix: %q", msg)
		}
		if !strings.Contains(msg, "boom") {
			t.Fatalf("panic msg missing wrapped err: %q", msg)
		}
	}()
	_ = mustStdlibContract(0, errors.New("boom"))
}

func TestMustLen_Passes(_ *testing.T) {
	mustLen("ok", 4, 4)
}

func TestMustLen_PanicsOnMismatch(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not string: %T", r)
		}
		if !strings.Contains(msg, "some-thing") {
			t.Fatalf("panic msg missing what: %q", msg)
		}
	}()
	mustLen("some-thing", 3, 4)
}
