package main

import (
	"net/http"
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
		name    string
		visible bool
		count   int
		want    bool
	}{
		{"every window closed", false, 0, true},
		{"a hidden window, which Wails shows", false, 1, false},
		{"a visible window", true, 1, false},
		{"visible but none left", true, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldOpenOnReopen(tc.visible, tc.count); got != tc.want {
				t.Errorf("shouldOpenOnReopen(%v, %d) = %v, want %v", tc.visible, tc.count, got, tc.want)
			}
		})
	}
}

func TestOpenWindowCount(t *testing.T) {
	a, b := &application.WebviewWindow{}, &application.WebviewWindow{}
	tests := []struct {
		name    string
		windows []application.Window
		closing application.Window
		want    int
	}{
		{"none", nil, nil, 0},
		{"nothing closing", []application.Window{a, b}, nil, 2},
		{"one closing", []application.Window{a, b}, a, 1},
		{"the only one closing", []application.Window{a}, a, 0},
		{"closing one already gone", []application.Window{b}, a, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := openWindowCount(tc.windows, tc.closing); got != tc.want {
				t.Errorf("openWindowCount = %d, want %d", got, tc.want)
			}
		})
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
	if got := m.focusedPath(); got != "" {
		t.Errorf("no handler (Clone from Git running): focusedPath = %q, want empty", got)
	}

	d.handler = http.NotFoundHandler()
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
