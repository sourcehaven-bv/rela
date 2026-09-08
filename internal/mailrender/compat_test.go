package mailrender_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// This file is the offline substitute for a paid client-rendering matrix
// (Litmus, Email on Acid). Those render real screenshots across real clients,
// but they need an account and a network round-trip, so they cannot gate a PR.
// The Can I Email dataset carries the same knowledge as data, so a rule that
// would have been caught by looking at a screenshot is caught by a unit test
// instead.
//
// The fixture is a PINNED, pruned copy of the dataset (testdata/caniemail.min.json,
// package version and update date recorded inside it). It is never fetched at
// test time: a test that reaches the network fails in CI for reasons unrelated
// to the code, and a compatibility floor that silently moves under you is worse
// than one that is a little stale.
//
// Treat it as a FLOOR, not a source of truth. A finding here is a real
// portability problem; the absence of a finding is not proof the mail looks
// right, and nothing in this file substitutes for opening the message in a
// client.
//
// To refresh the fixture, install the package and reduce it to the two fields
// used here — the upstream file is several MB, most of it prose and per-version
// history this check never reads:
//
//	npm install caniemail
//	python3 - <<'EOF'
//	import json
//	src = json.load(open("node_modules/caniemail/data/caniemail.json"))
//	out = {"_source": "https://www.caniemail.com/ (caniemail npm package)",
//	       "_package_version": "<the installed version>",
//	       "api_version": src["api_version"],
//	       "last_update_date": src["last_update_date"], "data": []}
//	for f in src["data"]:
//	    if not f["slug"].startswith(("css-", "html-")):
//	        continue
//	    stats = {}
//	    for client, plats in f["stats"].items():
//	        for plat, vers in plats.items():
//	            if vers:  # newest tested version wins; strip the "#note" suffix
//	                stats[f"{client}/{plat}"] = vers[sorted(vers)[-1]].split()[0]
//	    out["data"].append({"slug": f["slug"], "stats": stats})
//	json.dump(out, open("testdata/caniemail.min.json", "w"), indent=1, sort_keys=True)
//	EOF
//
// Expect churn in the diff when you do: verdicts move as clients change. Read
// what moved rather than accepting it wholesale.

// caniemailFixture is the pruned dataset: per feature, the worst-case verdict
// for each client/platform at its most recently tested version.
type caniemailFixture struct {
	PackageVersion string `json:"_package_version"`
	LastUpdateDate string `json:"last_update_date"`
	Data           []struct {
		Slug string `json:"slug"`
		// Stats is keyed "client/platform" and holds that client's verdict at
		// its most recently tested version: "y" supported, "a" partial, "n"
		// unsupported, "u" unknown.
		Stats map[string]string `json:"stats"`
	} `json:"data"`
}

func loadCanIEmail(t *testing.T) caniemailFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "caniemail.min.json"))
	require.NoError(t, err, "vendored Can I Email fixture is missing")

	var f caniemailFixture
	require.NoError(t, json.Unmarshal(raw, &f))
	require.NotEmpty(t, f.Data, "fixture carries no features")
	return f
}

// compatSample is the message the compatibility checks render. It deliberately
// exercises every structural branch of the template — intro prose, a linked
// data table, a markdown body (including a GFM table, which takes a different
// styling path from a Section table), an empty section, a footer, and a logo —
// because a rule that is only emitted on one branch is only checked if that
// branch renders.
func compatSample(t *testing.T) string {
	t.Helper()

	r, err := mailrender.New(&mailrender.Options{
		BaseURL:    "https://app.example",
		LogoCID:    "logo@rela",
		LogoAlt:    "Example",
		LogoWidth:  120,
		LogoHeight: 32,
	})
	require.NoError(t, err)

	html, _, err := r.Render(&mailrender.Message{
		Subject: "Weekly digest",
		Lang:    "en",
		Intro:   "You have **3** items. See [the board](https://app.example/board).",
		Sections: []mailrender.Section{{
			Title:   "Overdue",
			Body:    "These are _past_ due.",
			Columns: []string{"Title", "Due"},
			Rows:    [][]string{{"Ship the thing", "2026-08-01"}},
			Links:   []string{"https://app.example/e/TKT-1"},
		}, {
			Title: "Notes",
			Body:  "## Heading\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- one\n- two",
		}, {
			Title:   "Empty",
			Columns: []string{"Title"},
		}},
		Footer: "Sent by rela.",
	})
	require.NoError(t, err)
	return string(html)
}

// styledElement is one element in the rendered output that carries a style
// attribute, paired with the declarations on it.
type styledElement struct {
	tag   string
	decls map[string]string
}

var compatStyleAttrRe = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\b[^>]*?\sstyle="([^"]*)"`)

func styledElements(html string) []styledElement {
	// Only the document body: the @media block in <head> is not an inline
	// style and its declarations are guarded by the at-rule, not by client
	// support for the property on an element.
	if i := strings.Index(html, "</style>"); i >= 0 {
		html = html[i:]
	}

	var out []styledElement
	for _, m := range compatStyleAttrRe.FindAllStringSubmatch(html, -1) {
		decls := map[string]string{}
		for d := range strings.SplitSeq(m[2], ";") {
			k, v, ok := strings.Cut(d, ":")
			if !ok {
				continue
			}
			decls[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
		out = append(out, styledElement{tag: strings.ToLower(m[1]), decls: decls})
	}
	return out
}

// isZeroLength reports whether every component of a CSS length value is zero,
// so "0", "0 0", and "0px 0" all count while "8px 0" does not.
func isZeroLength(v string) bool {
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		n := strings.TrimRight(f, "pxemrt%") // px, em, rem, pt, %
		if n != "0" {
			return false
		}
	}
	return true
}

// TestCompat_PaddingOnlyOnTableCells is the ticket's headline finding (A1).
//
// Can I Email records padding as supported ONLY on table cells in Outlook
// Windows and Windows Mail. A padded <div> therefore renders correctly in every
// client a developer is likely to check and collapses in the one they are not,
// which is exactly the class of bug a dataset check exists to catch.
//
// Zero values are exempt wherever they appear. A "padding: 0" is a RESET —
// it overrides a client's default so that some other property controls the
// spacing — and a dropped zero cannot collapse into a visible defect. Only a
// non-zero padding is load-bearing, and only that is worth failing on.
func TestCompat_PaddingOnlyOnTableCells(t *testing.T) {
	t.Parallel()

	var offenders []string
	for _, el := range styledElements(compatSample(t)) {
		if el.tag == "td" || el.tag == "th" {
			continue
		}
		for prop, val := range el.decls {
			if prop != "padding" && !strings.HasPrefix(prop, "padding-") {
				continue
			}
			if isZeroLength(val) {
				continue
			}
			offenders = append(offenders, fmt.Sprintf("<%s> %s: %s", el.tag, prop, val))
		}
	}
	sort.Strings(offenders)

	require.Empty(t, offenders,
		"padding on a non-cell element is dropped by Outlook Windows "+
			"(Can I Email css-padding: \"Only supported on table cells\").\n"+
			"Move the padding onto a <td>, or wrap the content in a single-cell table.")
}

// TestCompat_NoLayoutMarginOnStructuralElements covers A2.
//
// margin is unsupported or partial across Gmail, Outlook, Yahoo and AOL, so it
// must not be the only thing producing a gap in the SCAFFOLDING. Margins inside
// sanitized markdown are a different case and are allowed: rela cannot inject
// spacer rows into author prose, and there a dropped margin costs a gap rather
// than a broken layout.
func TestCompat_NoLayoutMarginOnStructuralElements(t *testing.T) {
	t.Parallel()

	html := compatSample(t)

	// The section scaffolding must carry no margin at all.
	for _, m := range regexp.MustCompile(`<table class="(sect|tbl)"[^>]*style="([^"]*)"`).FindAllStringSubmatch(html, -1) {
		require.NotContains(t, m[2], "margin",
			"structural table .%s relies on margin, which Gmail and Outlook drop; use a spacer row", m[1])
	}

	// And the gap it uses instead must actually be present.
	require.Contains(t, html, `class="gap"`,
		"the spacer row that replaced the table margin is gone; tables will butt against the next heading")
}

// TestCompat_DataTableIsNotPresentational covers A3.
//
// role="presentation" tells assistive technology to ignore a table's structure.
// That is correct for the layout tables (.wrap, .card, .sect) which exist only
// to position things, and wrong for .tbl, which carries real <th> headers
// describing real columns.
func TestCompat_DataTableIsNotPresentational(t *testing.T) {
	t.Parallel()

	html := compatSample(t)

	for _, m := range regexp.MustCompile(`<table class="tbl"[^>]*>`).FindAllString(html, -1) {
		require.NotContains(t, m, `role="presentation"`,
			"the data table hides its headers from screen readers")
	}
	require.Contains(t, html, `scope="col"`, "data-table headers must declare their scope")

	// The layout tables keep the role — dropping it there would announce
	// meaningless structure instead.
	for _, cls := range []string{"wrap", "card"} {
		re := regexp.MustCompile(`<table class="` + cls + `"[^>]*>`)
		found := re.FindString(html)
		require.NotEmpty(t, found, "layout table .%s not found", cls)
		require.Contains(t, found, `role="presentation"`,
			"layout table .%s must stay presentational", cls)
	}
}

// TestCompat_NoColorSchemeMetaTag covers A5, and it is a guard against a
// plausible-looking "improvement" rather than against a typo.
//
// Adding <meta name="color-scheme"> looks like free dark-mode support. It is
// not: Litmus documents that Apple Mail applies a partial invert when the tag
// is present WITHOUT a full matching dark stylesheet. Apple Mail otherwise
// leaves this template alone, so the tag trades a correct rendering for a
// half-inverted one in a major client. See TKT-1GA2PG.
func TestCompat_NoColorSchemeMetaTag(t *testing.T) {
	t.Parallel()

	// Match the META TAG specifically. A substring search for "color-scheme"
	// would also hit the prefers-color-scheme media query, which is the thing
	// we deliberately DO ship.
	metas := regexp.MustCompile(`(?i)<meta[^>]*>`).FindAllString(compatSample(t), -1)
	for _, m := range metas {
		require.NotContains(t, strings.ToLower(m), "color-scheme",
			"the color-scheme meta tag opts Apple Mail into inverting this mail; "+
				"it is deliberately absent (TKT-1GA2PG)")
	}
}

// TestCompat_DarkModeBlockSurvivesInlining pins that the @media block reaches
// the recipient.
//
// douceur cannot inline an at-rule, so this rule can only work by SURVIVING in
// <head> — and a future change that starts sanitizing or rewriting the
// assembled document would silently drop it while every other test still
// passes.
func TestCompat_DarkModeBlockSurvivesInlining(t *testing.T) {
	t.Parallel()

	html := compatSample(t)
	require.Contains(t, html, "@media (prefers-color-scheme: dark)",
		"the dark-mode block did not survive CSS inlining")

	// The dark palette must actually be interpolated, not left as a literal.
	require.NotContains(t, html, "{{", "an unexpanded template action reached the output")
}

// TestCompat_LogoCarriesIntrinsicDimensions covers A6.
//
// Outlook Windows ignores max-height, so width/height attributes are the only
// constraint on a large logo there.
func TestCompat_LogoCarriesIntrinsicDimensions(t *testing.T) {
	t.Parallel()

	img := regexp.MustCompile(`<img[^>]*>`).FindString(compatSample(t))
	require.NotEmpty(t, img, "logo image not rendered")
	require.Contains(t, img, `width="120"`)
	require.Contains(t, img, `height="32"`)
}

// TestCompat_NoUnsupportedPropertyOnAnyElement is the broad sweep: every
// declaration actually emitted is scored against the dataset, and a property
// that no tested client supports fails.
//
// The bar is deliberately "unsupported EVERYWHERE tested" rather than
// "unsupported somewhere". Email CSS is a compromise — padding is partial in
// Outlook and still the only sane way to space a cell — so failing on any
// partial support would mean failing on a correct template. What this catches
// is a property that buys nothing at all.
func TestCompat_NoUnsupportedPropertyOnAnyElement(t *testing.T) {
	t.Parallel()

	fixture := loadCanIEmail(t)
	t.Logf("Can I Email fixture: package %s, data updated %s",
		fixture.PackageVersion, fixture.LastUpdateDate)

	// Age the fixture out loud rather than failing on it. Client behavior does
	// drift, but a date-triggered CI failure would break builds for a reason
	// nobody changed, so this is a prompt to run the refresh recipe in the file
	// header, not a gate.
	if d, err := time.Parse("2006-01-02", strings.Fields(fixture.LastUpdateDate)[0]); err == nil {
		if age := time.Since(d); age > staleAfter {
			t.Logf("NOTE: the compatibility dataset is %d months old — consider refreshing it "+
				"(recipe in this file's header comment)", int(age.Hours()/24/30))
		}
	}

	supported := map[string]bool{}
	for _, f := range fixture.Data {
		yes := false
		for _, verdict := range f.Stats {
			if verdict == "y" || verdict == "a" {
				yes = true
				break
			}
		}
		supported[f.Slug] = yes
	}

	seen := map[string]bool{}
	var dead, untracked []string
	for _, el := range styledElements(compatSample(t)) {
		for prop := range el.decls {
			slug := "css-" + strings.TrimPrefix(prop, "-")
			known, tracked := supported[slug]
			if seen[prop] {
				continue
			}
			seen[prop] = true
			if !tracked {
				// Shorthands like border-bottom have no dataset entry, so they
				// are scored against nothing. Counted and reported rather than
				// skipped silently: without this a reader reasonably assumes
				// every emitted declaration was checked.
				untracked = append(untracked, prop)
				continue
			}
			if !known {
				dead = append(dead, prop)
			}
		}
	}
	sort.Strings(dead)
	sort.Strings(untracked)
	t.Logf("scored %d properties against the dataset; %d untracked (%s)",
		len(seen)-len(untracked), len(untracked), strings.Join(untracked, ", "))

	require.Empty(t, dead,
		"these CSS properties are unsupported in every client Can I Email tests, "+
			"so they cost bytes and buy nothing: %v", dead)
}

// TestCompat_GuardActuallyFires is the test for the tests.
//
// A compatibility check that cannot fail is worse than no check, because it
// reads like coverage. This feeds the padding rule a document that violates it
// and asserts the detection logic reports it — so if styledElements ever stops
// parsing the real output (a quoting change, say), this fails rather than
// quietly passing everything.
func TestCompat_GuardActuallyFires(t *testing.T) {
	t.Parallel()

	bad := `<html><body style="padding: 0;">` +
		`<div style="padding: 12px; color: #333;">flagged</div>` +
		`<ul style="padding-left: 0;">reset, not flagged</ul>` +
		`<td style="padding: 8px;">cell, not flagged</td></body></html>`

	var offenders []string
	for _, el := range styledElements(bad) {
		if el.tag == "td" || el.tag == "th" {
			continue
		}
		for prop, val := range el.decls {
			if strings.HasPrefix(prop, "padding") && !isZeroLength(val) {
				offenders = append(offenders, el.tag)
			}
		}
	}

	require.Equal(t, []string{"div"}, offenders,
		"the padding guard must flag the padded <div> and only that: a guard that "+
			"cannot fire would not catch a regression either")

	require.True(t, isZeroLength("0") && isZeroLength("0px 0"), "zero resets must be exempt")
	require.False(t, isZeroLength("8px 0"), "a real padding must not be treated as a reset")
}

// TestCompat_DarkModeDoesNotEraseTheAccentBar guards a bug found by LOOKING at
// the rendered mail rather than by any assertion.
//
// The dark rule was originally written as ".wrap td", a descendant selector
// that reaches EVERY cell inside the wrapper — including the 4px accent bar at
// the top of the card, which it repainted in the background color, silently
// deleting the one piece of brand color in the layout.
//
// Nothing else catches this: the block is present, the colors are valid, and
// every structural test passes. Only an assertion that the bar keeps a color
// DISTINCT from the background does.
func TestCompat_DarkModeDoesNotEraseTheAccentBar(t *testing.T) {
	t.Parallel()

	html := compatSample(t)
	start := strings.Index(html, "@media (prefers-color-scheme: dark)")
	require.GreaterOrEqual(t, start, 0, "dark block missing")
	dark := html[start:strings.Index(html, "</style>")]

	require.NotContains(t, dark, ".wrap td",
		"a descendant selector on .wrap repaints the accent bar with the page "+
			"background and erases it; target the specific cells instead")

	// The bar must be styled, and to something other than the page background.
	require.Contains(t, dark, ".bar", "the accent bar has no dark-mode color")

	barRule := regexp.MustCompile(`\.bar \{[^}]*background-color:\s*([^;!]+)`).FindStringSubmatch(dark)
	require.Len(t, barRule, 2, "could not read the dark .bar color")

	wrapRule := regexp.MustCompile(`\.wrap \{[^}]*background-color:\s*([^;!]+)`).FindStringSubmatch(dark)
	require.Len(t, wrapRule, 2, "could not read the dark .wrap color")

	require.NotEqual(t, strings.TrimSpace(wrapRule[1]), strings.TrimSpace(barRule[1]),
		"the accent bar is the same color as the background, so it is invisible")
}

// staleAfter is when the vendored dataset starts drawing a log note. Twelve
// months: long enough that a healthy repo never sees it, short enough that a
// genuinely abandoned fixture says so.
const staleAfter = 365 * 24 * time.Hour
