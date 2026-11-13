package lua

import "testing"

func TestBuildScriptError_RetryFalseIsPermanent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, code, msg string
		permanent       bool
	}{
		{"retry false", `error({message = "bad input", retry = false})`, "bad input", true},
		{"retry true", `error({message = "busy", retry = true})`, "busy", false},
		{"no retry field", `error({message = "busy"})`, "busy", false},
		{"retry not a boolean", `error({message = "busy", retry = "no"})`, "busy", false},
		{"string error", `error("plain")`, "plain", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			frames, err := loadAndRun(t, "scripts/job.lua", tc.code)
			if err == nil {
				t.Fatal("expected error")
			}
			se := BuildScriptError(BuildInput{Surface: SurfaceAutomation, Path: "scripts/job.lua", Err: err,
				Frames: frames})
			if se.Permanent != tc.permanent {
				t.Errorf("Permanent=%v, want %v", se.Permanent, tc.permanent)
			}
			if se.LuaMessage != tc.msg {
				t.Errorf("LuaMessage=%q, want %q", se.LuaMessage, tc.msg)
			}
		})
	}
}
