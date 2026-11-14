package main

import (
	"slices"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

func TestMainWindowOptions(t *testing.T) {
	tests := []struct {
		name      string
		ws        desktop.WindowState
		wantW     int
		wantH     int
		wantPos   application.WindowStartPosition
		wantState application.WindowState
	}{
		{"no saved state", desktop.WindowState{}, defaultWindowWidth, defaultWindowHeight, 0, 0},
		{"degenerate state", desktop.WindowState{Width: 10, Height: 10, X: 5}, defaultWindowWidth, defaultWindowHeight, 0, 0},
		{"saved state", desktop.WindowState{Width: 900, Height: 700, X: 40, Y: 50}, 900, 700, application.WindowXY, 0},
		{
			"maximized", desktop.WindowState{Width: 900, Height: 700, Maximized: true},
			900, 700, application.WindowXY, application.WindowStateMaximised,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mainWindowOptions("Proj", tc.ws)
			if got.Name != mainWindowName || got.Title != "Proj" {
				t.Errorf("name/title = %q/%q", got.Name, got.Title)
			}
			if got.Width != tc.wantW || got.Height != tc.wantH {
				t.Errorf("size = %dx%d, want %dx%d", got.Width, got.Height, tc.wantW, tc.wantH)
			}
			if got.InitialPosition != tc.wantPos {
				t.Errorf("InitialPosition = %v, want %v", got.InitialPosition, tc.wantPos)
			}
			if tc.wantPos == application.WindowXY && (got.X != tc.ws.X || got.Y != tc.ws.Y) {
				t.Errorf("position = %d,%d, want %d,%d", got.X, got.Y, tc.ws.X, tc.ws.Y)
			}
			if got.StartState != tc.wantState {
				t.Errorf("StartState = %v, want %v", got.StartState, tc.wantState)
			}
		})
	}
}

func TestShouldOpenOnReopen(t *testing.T) {
	tests := []struct {
		visible bool
		count   int
		want    bool
	}{
		{false, 0, true},  // every window closed: open one
		{false, 1, false}, // a hidden window: Wails shows it
		{true, 1, false},  // a window is already there
	}
	for _, tc := range tests {
		if got := shouldOpenOnReopen(tc.visible, tc.count); got != tc.want {
			t.Errorf("shouldOpenOnReopen(%v, %d) = %v, want %v", tc.visible, tc.count, got, tc.want)
		}
	}
}

func TestRevealArgs(t *testing.T) {
	got := revealArgs("-odd/path")
	if want := []string{"-R", "--", "-odd/path"}; !slices.Equal(got, want) {
		t.Errorf("revealArgs = %v, want %v", got, want)
	}
}

func TestRevealInFinder_RefusesNoPath(t *testing.T) {
	if err := revealInFinder(""); err == nil {
		t.Error("revealInFinder(\"\") = nil, want an error")
	}
	if err := revealInFinder(t.TempDir() + "/missing"); err == nil {
		t.Error("revealInFinder(missing) = nil, want an error")
	}
}

func TestFocusedPath(t *testing.T) {
	d := &Desktop{prefs: &desktop.Preferences{}, registry: newProjectRegistry()}
	m := &menuBar{d: d}

	if got := m.focusedPath(); got != "" {
		t.Errorf("no project: focusedPath = %q, want empty", got)
	}

	d.app = &dataentry.App{}
	d.activePath = "/work/active.rela"
	if got := m.focusedPath(); got != "/work/active.rela" {
		t.Errorf("active project: focusedPath = %q", got)
	}

	d.registry.add(&loadedProject{id: "abc", root: "/work/other"})
	id := "abc"
	m.focused.Store(&id)
	if got := m.focusedPath(); got != "/work/other" {
		t.Errorf("focused project: focusedPath = %q", got)
	}

	gone := "gone"
	m.focused.Store(&gone)
	if got := m.focusedPath(); got != "/work/active.rela" {
		t.Errorf("unknown focused project: focusedPath = %q, want the active one", got)
	}
}
