package ai

import "testing"

func TestRedactKey_Empty(t *testing.T) {
	if got := redactKey("hello sk-secret world", ""); got != "hello sk-secret world" {
		t.Errorf("empty key should pass through, got %q", got)
	}
}

func TestRedactKey_Simple(t *testing.T) {
	got := redactKey("auth failed for sk-abc123", "sk-abc123")
	if got != "auth failed for <REDACTED>" {
		t.Errorf("got %q", got)
	}
}

func TestRedactKey_Bearer(t *testing.T) {
	got := redactKey("header was: Bearer sk-abc123 (rejected)", "sk-abc123")
	if got != "header was: <REDACTED> (rejected)" {
		t.Errorf("got %q", got)
	}
}

func TestRedactKey_MultipleOccurrences(t *testing.T) {
	got := redactKey("sk-abc and again sk-abc", "sk-abc")
	if got != "<REDACTED> and again <REDACTED>" {
		t.Errorf("got %q", got)
	}
}

func TestRedactKey_NotPresent(t *testing.T) {
	got := redactKey("nothing to redact", "sk-secret")
	if got != "nothing to redact" {
		t.Errorf("got %q", got)
	}
}
