package cli

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

func TestFormatScriptError_NilReturnsEmpty(t *testing.T) {
	if got := formatScriptError(nil); got != "" {
		t.Errorf("got %q, want empty string for nil", got)
	}
}

func TestFormatScriptError_HeadlineOnlyWhenNoSource(t *testing.T) {
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo",
		LuaMessage: "boom",
	}
	got := formatScriptError(se)
	want := "validations/foo: boom"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatScriptError_HeadlineWithLine(t *testing.T) {
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo",
		LuaMessage: "boom",
		LuaLine:    42,
	}
	got := formatScriptError(se)
	want := "validations/foo:42: boom"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatScriptError_RendersSourceSliceWithHighlight(t *testing.T) {
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo.lua",
		LuaMessage: "attempt to index a nil value",
		LuaLine:    4,
		Source: []lua.SourceLine{
			{N: 2, Text: "local x"},
			{N: 3, Text: "local y"},
			{N: 4, Text: "return y.field", Highlight: true},
			{N: 5, Text: "-- after"},
		},
	}
	got := formatScriptError(se)

	if !strings.HasPrefix(got, "validations/foo.lua:4: attempt to index a nil value") {
		t.Errorf("missing headline; got: %q", got)
	}
	if !strings.Contains(got, "  >    4 | return y.field") {
		t.Errorf("missing highlighted failing line; got:\n%s", got)
	}
	if !strings.Contains(got, "       3 | local y") {
		t.Errorf("missing context line 3; got:\n%s", got)
	}
}

func TestFormatScriptError_CollapsesMultiLineHeadline(t *testing.T) {
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo",
		LuaMessage: "context deadline exceeded\nat 5s\nin loop",
	}
	got := formatScriptError(se)

	if strings.Contains(got, "\n") {
		t.Errorf("got embedded newline in headline: %q", got)
	}
	if !strings.Contains(got, "context deadline exceeded at 5s in loop") {
		t.Errorf("expected newlines collapsed to spaces; got: %q", got)
	}
	if len(got) > scriptErrorMessageMaxLen+5 {
		t.Errorf("got len %d, want at most %d", len(got), scriptErrorMessageMaxLen+5)
	}
}

func TestFormatScriptError_CollapsesMultiLineThenTruncates(t *testing.T) {
	long := strings.Repeat("a\n", scriptErrorMessageMaxLen)
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo",
		LuaMessage: long,
	}
	got := formatScriptError(se)

	if strings.Contains(got, "\n") {
		t.Errorf("got embedded newline after collapse: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncation marker; got: %q", got[len(got)-10:])
	}
}

func TestFormatScriptError_TruncatesVeryLongHeadline(t *testing.T) {
	long := strings.Repeat("a", scriptErrorMessageMaxLen+50)
	se := &lua.ScriptError{
		Surface:    lua.SurfaceValidation,
		Path:       "validations/foo",
		LuaMessage: long,
	}
	got := formatScriptError(se)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncation marker; got: %q", got[len(got)-10:])
	}
	if len(got) > scriptErrorMessageMaxLen+5 { // +5 for "..." marker
		t.Errorf("got len %d, want at most %d", len(got), scriptErrorMessageMaxLen+5)
	}
}
