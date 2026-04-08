package dataentryconfig

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGenerateDarkParityGoldens dumps the output of generateDark and
// generateDarkBadges for a fixed set of input palettes. It serves as
// a permanent parity contract between the Go reference implementation
// and the TypeScript port in frontend/src/utils/palette.ts (which
// reads the same JSON file from
// frontend/src/utils/__fixtures__/generate_dark_goldens.json).
//
// If you intentionally change the dark generation algorithm:
//  1. Run `UPDATE_GOLDENS=1 go test ./internal/dataentryconfig/ -run TestGenerateDarkParityGoldens`
//  2. Copy the regenerated file to the frontend fixtures dir.
//  3. Run the frontend palette.test.ts to confirm the TS port matches.
func TestGenerateDarkParityGoldens(t *testing.T) {
	type fixture struct {
		Name   string            `json:"name"`
		Light  PaletteColors     `json:"light"`
		Badges map[string]string `json:"badges"`
	}

	type goldenEntry struct {
		Name       string            `json:"name"`
		Light      PaletteColors     `json:"light"`
		Badges     map[string]string `json:"badges"`
		Dark       PaletteColors     `json:"dark"`
		DarkBadges map[string]string `json:"darkBadges"`
	}

	fixtures := []fixture{
		{
			Name: "framework_defaults",
			Light: PaletteColors{
				Base: "#1a1a2e", Surface: "#f8fafc", Accent: "#6366f1", Text: "#1e293b",
				Success: "#10b981", Error: "#ef4444", Warning: "#f59e0b", Info: "#3b82f6",
			},
			Badges: map[string]string{
				"blue": "#3b82f6", "purple": "#8b5cf6", "green": "#22c55e", "gray": "#6b7280",
				"red": "#ef4444", "orange": "#f97316", "yellow": "#eab308",
			},
		},
		{
			Name: "lospec_sweetie16",
			Light: PaletteColors{
				Base: "#1a1c2c", Surface: "#f4f4f4", Accent: "#ffcd75", Text: "#5d275d",
				Success: "#38b764", Error: "#ef7d57", Warning: "#a7f070", Info: "#41a6f6",
			},
			Badges: map[string]string{
				"blue": "#41a6f6", "purple": "#3b5dc9", "green": "#38b764", "gray": "#94b0c2",
				"red": "#b13e53", "orange": "#ef7d57", "yellow": "#ffcd75",
			},
		},
		{
			Name: "high_contrast_purple",
			Light: PaletteColors{
				Base: "#0f0f1e", Surface: "#fafafa", Accent: "#7c3aed", Text: "#0f172a",
				Success: "#16a34a", Error: "#dc2626", Warning: "#ea580c", Info: "#2563eb",
			},
			Badges: map[string]string{
				"blue": "#2563eb", "purple": "#7c3aed", "green": "#16a34a", "gray": "#64748b",
				"red": "#dc2626", "orange": "#ea580c", "yellow": "#ca8a04",
			},
		},
		{
			Name: "near_pure_black_white",
			Light: PaletteColors{
				Base: "#000000", Surface: "#ffffff", Accent: "#ff00ff", Text: "#000000",
				Success: "#00ff00", Error: "#ff0000", Warning: "#ffff00", Info: "#0000ff",
			},
			Badges: map[string]string{
				"blue": "#0000ff", "purple": "#8000ff", "green": "#00ff00", "gray": "#808080",
				"red": "#ff0000", "orange": "#ff8000", "yellow": "#ffff00",
			},
		},
	}

	out := make([]goldenEntry, 0, len(fixtures))
	for _, f := range fixtures {
		out = append(out, goldenEntry{
			Name:       f.Name,
			Light:      f.Light,
			Badges:     f.Badges,
			Dark:       generateDark(f.Light),
			DarkBadges: generateDarkBadges(f.Badges),
		})
	}

	const goldenPath = "testdata/generate_dark_goldens.json"
	encoded, err := json.MarshalIndent(out, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')

	if os.Getenv("UPDATE_GOLDENS") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, encoded, 0o644))
		return
	}

	existing, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("missing golden file %s — run with UPDATE_GOLDENS=1 to create: %v", goldenPath, err)
	}
	require.Equal(t, string(existing), string(encoded), "generateDark output drifted from goldens; if intentional, regenerate with UPDATE_GOLDENS=1")
}
