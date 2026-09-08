package docs

import (
	"fmt"
	"sort"
	"strings"
)

// Region is a NAME a manual may address a part of the rendered page by.
//
// # Why a closed vocabulary and not a selector
//
// The one thing this table buys is that a doc author can never write
// `.kanban-card`. A manual carrying CSS selectors is a second, undeclared
// consumer of the SPA's markup: a class rename turns every figure's assertion
// red at once, and — far worse — a class that merely stops MATCHING turns them
// green, because a selector that resolves to nothing looks exactly like a
// region with nothing in it. That is the vacuous-pass shape this whole feature
// exists to remove, so the mapping lives HERE, in one Go table, and the
// language exposes only names.
//
// It also means the markup contract is reviewable. When the accessibility pass
// renamed things, exactly one file had to change.
type Region struct {
	// Name is the vocabulary word an author writes: region="kanban-card".
	Name string
	// Selector is the DOM query the capture layer runs. It is INTERNAL — it is
	// never parsed from a manual, only looked up here.
	Selector string
	// Multiple marks a region that legitimately resolves to many elements
	// (cards, rows, result items). It changes only what a failure says: a
	// singular region reporting "no element" is a different diagnosis from a
	// list that is genuinely empty.
	Multiple bool
	// Why documents the anchor choice, and is REQUIRED to be non-empty for any
	// entry that falls back to a class name (enforced by TestRegionAnchors).
	// An ARIA role/name anchor is self-justifying; a class is a debt and says
	// so at the entry.
	Why string
}

// regions is the single name → DOM mapping. Nothing else in the codebase may
// know a doc-language region's markup.
//
// Anchors prefer ROLE + ACCESSIBLE NAME over class names, so the vocabulary
// rides markup that is independently correct: an assertion breaks when the
// page stops being accessible, which is a bug worth failing on either way. A
// class fallback is used only where no honest role exists, and each one says
// why at its entry.
var regions = []Region{
	{
		Name:     "menu",
		Selector: `aside#main-sidebar`,
		Why: "The sidebar <nav> is the only nav landmark, but it holds only the " +
			"CONFIGURED groups — Search and Analysis sit in a sibling div above it. " +
			"A manual asserting 'the menu offers Policies' means the whole rail, so " +
			"the region is the <aside> landmark that contains both.",
	},
	{
		Name:     "main",
		Selector: `main`,
		Why:      "The <main> landmark: exactly one per page, named by the standard.",
	},
	{
		Name:     "list",
		Selector: `table[aria-labelledby="entity-list-heading"]`,
		Why:      "The list table is named by the view's <h1>, so this is role+name, not a class.",
	},
	{
		Name:     "table-row",
		Selector: `tr.entity-row`,
		Multiple: true,
		Why: "CLASS FALLBACK. A data row has no role of its own that distinguishes it " +
			"from the header row, and rows are named only by their cells. `.entity-row` " +
			"is the row marker the SPA already keys `data-entity-id` off, so it is the " +
			"same contract the app itself relies on rather than a new one.",
	},
	{
		Name:     "kanban",
		Selector: `[role="group"][aria-label$=" board"]`,
		Why:      "The board is role=group named '<title> board' (KanbanView, both swimlane and flat layouts).",
	},
	{
		Name:     "kanban-column",
		Selector: `section.kanban-column`,
		Multiple: true,
		Why: "CLASS FALLBACK. Columns are <section aria-labelledby=kanban-col-*>, but an " +
			"accessible-name match would need a per-column value the author does not " +
			"know. The class is scoped inside the board region above.",
	},
	{
		Name:     "kanban-card",
		Selector: `a.kanban-card`,
		Multiple: true,
		Why: "A card is a real LINK with aria-label = the card title (TKT-3CSZRG); it " +
			"was `[role=button]` until cards became anchors, which is a stronger " +
			"contract — an <a> is focusable and activatable natively and supports " +
			"cmd/middle-click. CLASS FALLBACK for the SCOPE only: a board also contains " +
			"other anchors (the create link, column headings), and no role distinguishes " +
			"a card from them, so `.kanban-card` narrows the element rather than " +
			"replacing it.",
	},
	{
		Name:     "detail",
		Selector: `main`,
		Why:      "A detail page IS the <main> landmark; there is no narrower wrapper worth naming.",
	},
	{
		Name:     "detail-section",
		Selector: `main section[aria-labelledby$="-heading"], main section[aria-label]`,
		Multiple: true,
		Why: "Detail sections are named either by their <h2 id=*-heading> or, when the " +
			"section renders its own heading, by aria-label. Both arms are accessible " +
			"names — no class involved.",
	},
	{
		Name:     "search-results",
		Selector: `section[aria-labelledby="search-results-heading"]`,
		Why:      "The results section is named by its count paragraph; role+name.",
	},
	{
		Name:     "search-result",
		Selector: `.result-item`,
		Multiple: true,
		Why: "CLASS FALLBACK. A result is a plain <li> inside the results list with no " +
			"role or name of its own; the list itself is the named landmark above.",
	},
	{
		Name:     "analyze",
		Selector: `section[aria-labelledby^="check-"]`,
		Multiple: true,
		Why:      "One <section> per check, named by <h3 id=check-<key>-title>; role+name.",
	},
	{
		Name:     "dashboard",
		Selector: `section[aria-labelledby^="dashboard-card-"]`,
		Multiple: true,
		Why: "One <section> per card, named by <h3 id=dashboard-card-<key>-title>. There is " +
			"no page-level dashboard landmark other than <main>, so the dashboard is " +
			"addressable as its CARDS — which is also the granularity a claim is made at.",
	},
	{
		Name:     "next-action",
		Selector: `section[aria-label="Suggested next action"]`,
		Why: "The suggestion card names itself with aria-label=\"Suggested next action\"; " +
			"role+name, no class. Deliberately NOT the `banner` region: a suggestion is " +
			"advisory page furniture, not a live region, and folding the two together " +
			"would let a claim about one pass on the other.",
	},
	{
		Name:     "banner",
		Selector: `[role="alert"], [role="status"]`,
		Multiple: true,
		Why: "Live-region roles are exactly the 'the page is telling you something' " +
			"surface (the kanban truncation banner, analyze's truncation notice).",
	},
	{
		Name:     "badge",
		Selector: `.world-badge`,
		Multiple: true,
		Why: "CLASS FALLBACK. A world badge is decorative-adjacent inline text with no " +
			"role; giving it one would be inventing markup to suit a test. This is the " +
			"region TKT-PI17Z6 is asserted through, so the class is load-bearing and " +
			"pinned by TestRegionAnchors.",
	},
}

// regionByName indexes [regions] for lookup, built once.
var regionByName = func() map[string]Region {
	m := make(map[string]Region, len(regions))
	for _, r := range regions {
		m[r.Name] = r
	}
	return m
}()

// lookupRegion resolves a doc-language region name.
//
// An unknown name is an error naming the valid set, the same way
// rejectUnknownKeys refuses a typo'd key: a region that resolves to nothing
// would make every `absent=` claim pass for the wrong reason, so a typo must
// never reach the browser.
func lookupRegion(name string) (Region, error) {
	r, ok := regionByName[name]
	if !ok {
		return Region{}, fmt.Errorf(
			"unknown region %q — a region is a NAME from a closed set, never a CSS "+
				"selector. Known regions: %s", name, strings.Join(regionNames(), ", "))
	}
	return r, nil
}

// regionNames lists the vocabulary, sorted, for a failure message.
func regionNames() []string {
	out := make([]string, 0, len(regions))
	for _, r := range regions {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}
