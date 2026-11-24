package dataentryconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateAppLanguage(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr string
	}{
		{name: "empty follows the browser", tag: ""},
		{name: "english", tag: "en"},
		{name: "dutch", tag: "nl"},
		{name: "dutch with region", tag: "nl-BE"},
		{name: "case is not significant", tag: "NL-nl"},
		{name: "not a tag", tag: "dutch!", wantErr: "not a valid BCP 47 language tag"},
		{name: "empty subtag", tag: "nl-", wantErr: "not a valid BCP 47 language tag"},
		// language.Parse accepts these; the browser does not, so the SPA
		// would ignore the setting.
		{name: "underscore separator", tag: "nl_NL", wantErr: "not a valid BCP 47 language tag"},
		{name: "unicode extension", tag: "nl-BE-u-ca-buddhist", wantErr: "extension or private-use"},
		{name: "private use", tag: "nl-x-test", wantErr: "extension or private-use"},
		{name: "script", tag: "nl-Latn-NL"},
		{name: "no message file", tag: "de", wantErr: "has no interface translation"},
		{name: "no message file with region", tag: "fr-BE", wantErr: "has no interface translation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateAppLanguage(tc.tag)
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("validateAppLanguage(%q) = %v, want no errors", tc.tag, errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], tc.wantErr) || !strings.HasPrefix(errs[0], "app.language: ") {
				t.Fatalf("validateAppLanguage(%q) = %v, want one app.language error containing %q",
					tc.tag, errs, tc.wantErr)
			}
		})
	}
}

// TestValidateApp_Language checks that validateApp reports app.language, so
// a bad tag fails the config load rather than only the helper.
func TestValidateApp_Language(t *testing.T) {
	cfg := &Config{App: AppConfig{Language: "de"}}
	errs := validateApp(cfg, nil)
	if len(errs) != 1 || !strings.HasPrefix(errs[0], "app.language: ") {
		t.Fatalf("validateApp(language=de) = %v, want one app.language error", errs)
	}
}

func TestValidateAppLanguages(t *testing.T) {
	tests := []struct {
		name    string
		langs   []string
		def     string
		wantErr []string
	}{
		{name: "empty allows all"},
		{name: "one", langs: []string{"en"}},
		{name: "both", langs: []string{"nl", "en"}, def: "nl-BE"},
		{name: "unsupported", langs: []string{"de"}, wantErr: []string{"app.languages: \"de\" is not a supported"}},
		{name: "region", langs: []string{"nl-BE"}, wantErr: []string{"app.languages: \"nl-BE\" is not a supported"}},
		{name: "upper case", langs: []string{"EN"}, wantErr: []string{"app.languages: \"EN\" is not a supported"}},
		{name: "twice", langs: []string{"en", "en"}, wantErr: []string{"app.languages: \"en\" is listed twice"}},
		{name: "default outside", langs: []string{"en"}, def: "nl", wantErr: []string{"app.language: \"nl\" is not one of app.languages"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateAppLanguages(tc.langs, tc.def)
			if len(errs) != len(tc.wantErr) {
				t.Fatalf("validateAppLanguages(%v, %q) = %v, want %d errors", tc.langs, tc.def, errs, len(tc.wantErr))
			}
			for i, want := range tc.wantErr {
				if !strings.HasPrefix(errs[i], want) {
					t.Errorf("error %d = %q, want prefix %q", i, errs[i], want)
				}
			}
		})
	}
}

// TestValidateApp_Languages checks that validateApp reports app.languages.
func TestValidateApp_Languages(t *testing.T) {
	cfg := &Config{App: AppConfig{Languages: []string{"de"}}}
	errs := validateApp(cfg, nil)
	if len(errs) != 1 || !strings.HasPrefix(errs[0], "app.languages: ") {
		t.Fatalf("validateApp(languages=[de]) = %v, want one app.languages error", errs)
	}
}

// TestSupportedUILanguagesMatchLocaleFiles pins SupportedUILanguages to the
// SPA's message files on disk. A language added on one side only would either
// pass validation and render English, or never be selectable from config.
func TestSupportedUILanguagesMatchLocaleFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "frontend", "src", "i18n", "locales")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var onDisk []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".json"); ok && !e.IsDir() {
			onDisk = append(onDisk, name)
		}
	}
	want := slices.Clone(SupportedUILanguages)
	slices.Sort(want)
	slices.Sort(onDisk)
	if !slices.Equal(onDisk, want) {
		t.Fatalf("locale files %v, SupportedUILanguages %v: add or remove the language on both sides", onDisk, want)
	}
}
