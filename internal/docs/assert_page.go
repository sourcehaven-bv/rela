package docs

import (
	"errors"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// pageKeys is every key page{} accepts. `count` is deliberately absent — it
// was removed, and TestCountIsNotAKey scans THIS list so it cannot creep back.
var pageKeys = []string{
	"view", "type", "entity", "form", "list", "q", "world", "as",
	"region", "menu_has", "has", "absent", "has_card", "card_absent", "emit",
}

// page{} asserts what a manual's prose claims about the RENDERED page.
//
// # Why screenshot{} was not enough
//
// `screenshot{}` asserts nothing. It proves a page loaded without erroring and
// writes a PNG; whether the PNG shows what the paragraph beside it promises is
// checked by a human looking at an image, once, at review time. Two real bugs
// shipped into the worlds manual behind green builds that way — a badge
// printing a world name where a face name belonged (TKT-PI17Z6), and kanban
// cards with no world badge at all under prose claiming the board distinguishes
// a fallback from a real card (TKT-ILT1WD). Both figures were captured
// successfully. Both captions were wrong.
//
// An executable manual that can be confidently wrong about its own figures is
// worth less than one that cannot be, so `page{}` puts the claim in the build.
//
// # It shares screenshot{}'s plumbing on purpose
//
// Standing up the seeded temp project, the SPA and a browser is the expensive
// part, and it is identical. `page{}` rides the same [CaptureSpec] and the same
// readiness gate through [PageInspector], so a manual that already screenshots
// a screen pays almost nothing to also assert about it.
//
// # It sees what earlier islands wrote
//
// page{} reads the document's ONE shared temp project, the same one api{} and
// screenshot{} use, and islands run top-to-bottom — so an assertion here can
// be about the result of a write an earlier api{} island made. See the
// ordering notes on [docRuntime.luaAPI] for the limits.
//
// # A call that asserts nothing is an ERROR
//
// The house rule every other verb here enforces. `page{view="kanban",
// list="pipeline"}` looks like a check and claims only that the page rendered,
// which is precisely the failure mode this verb exists to kill.
//
// # Why these are free functions, not methods on docRuntime
//
// docRuntime is already over the plimsoll method load line (TKT-N0IKN9 tracks
// decomposing it). A new verb does not need to make that worse: taking dr as a
// parameter is the same code with none of the accretion, and it keeps the
// runtime's surface flat the way registerModule's closures already do.
func luaPage(dr *docRuntime, ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, `page: expects a table, e.g. `+
			`page{view="kanban", list="pipeline", has_card={"Data Retention"}}`)
	}
	if dr.tierB.capturer == nil {
		reason := dr.tierB.capturerErr
		if reason == "" {
			reason = "page{} needs a Chrome/Chromium browser and a built data-entry SPA"
		}
		return dr.luaFail(ls, "page: no browser capturer available — %s", reason)
	}
	inspector, ok := dr.tierB.capturer.(PageInspector)
	if !ok {
		return dr.luaFail(ls, "page: the wired capturer cannot inspect page content "+
			"(it implements Capturer but not PageInspector)")
	}

	if rejectUnknownKeys(dr, ls, "page", tbl, pageKeys...) {
		return 0
	}
	show := fieldBoolDefault(ls, tbl, "emit", true)

	spec, failed := pageSpec(dr, ls, tbl)
	if failed {
		return 0
	}

	claims, err := readPageClaims(ls, tbl)
	if err != nil {
		return dr.luaFail(ls, "page: %v", err)
	}
	if err := claims.validate(); err != nil {
		return dr.luaFail(ls, "page{view=%q}: %v", spec.View, err)
	}

	// One browser trip per REGION named, not per claim: menu_has and a
	// region= claim address different parts of the same render, and asking
	// twice would photograph two different moments of a live SPA.
	texts := map[string][]string{}
	for _, name := range claims.regionsUsed() {
		region, rerr := lookupRegion(name)
		if rerr != nil {
			return dr.luaFail(ls, "page: %v", rerr)
		}
		got, terr := inspector.Inspect(dr.ctx, spec, region.Selector)
		if terr != nil {
			return dr.luaFail(ls, "page{view=%q, region=%q}: %v", spec.View, name, terr)
		}
		texts[name] = got
	}

	if msg := claims.check(spec.View, texts); msg != "" {
		return dr.luaFail(ls, "%s", msg)
	}

	emitEvidence(dr.emit, show, pageEvidence(spec, claims))
	return 0
}

// pageEvidence captions what the screen was checked for.
//
// Deliberately terser than the other verbs: page{} sits beside a screenshot,
// and the figure already shows the screen. What the image CANNOT say is that a
// machine confirmed the caption — a reviewer looking at a PNG cannot tell an
// asserted figure from a decorative one, and both of the bugs this verb was
// written for (TKT-PI17Z6, TKT-ILT1WD) were captured successfully with wrong
// captions.
func pageEvidence(spec CaptureSpec, claims pageClaims) evidence {
	where := fmt.Sprintf("the `%s` screen", spec.View)
	if spec.World != "" {
		where = fmt.Sprintf("%s in %s", where, worldPhrase(spec.World))
	}
	ev := evidence{claim: fmt.Sprintf("On %s, the screenshot above was checked, not just captured.", where)}
	if absent := claims.absentTexts(); len(absent) > 0 {
		ev.note = fmt.Sprintf("Confirmed NOT on the screen: %s.", strings.Join(absent, ", "))
	}
	return ev
}

// pageSpec builds the CaptureSpec for a page{} call, reusing screenshot{}'s
// per-view argument rules so the two verbs cannot disagree about what names a
// screen. Returns failed=true when it has already raised.
func pageSpec(dr *docRuntime, ls *lua.LState, tbl *lua.LTable) (CaptureSpec, bool) {
	if w := fieldString(ls, tbl, "world"); w != "" {
		if _, err := dr.worldScope(w); err != nil {
			dr.luaFail(ls, "page: %v", err)
			return CaptureSpec{}, true
		}
	}
	spec := CaptureSpec{
		ProjectDir: dr.tierB.projectDir,
		Seed:       dr.seed.ops,
		View:       fieldStringDefault(ls, tbl, "view", "list"),
		Type:       fieldString(ls, tbl, "type"),
		Entity:     fieldString(ls, tbl, "entity"),
		Form:       fieldString(ls, tbl, "form"),
		As:         fieldString(ls, tbl, "as"),
		World:      fieldString(ls, tbl, "world"),
		List:       fieldString(ls, tbl, "list"),
		Query:      fieldString(ls, tbl, "q"),
	}
	if !supportedView(spec.View) {
		dr.luaFail(ls, "page: unknown view %q — one of %s", spec.View, strings.Join(supportedViews, ", "))
		return CaptureSpec{}, true
	}
	if err := requireViewArgs(spec); err != nil {
		dr.luaFail(ls, "page: %v", err)
		return CaptureSpec{}, true
	}
	return spec, false
}

// pageClaims is one page{} call's assertions, split by the region each
// addresses. Split from the Lua binding so the RULES and the FAILURE TEXT are
// testable without a browser — a doctest's value is its failure output, and
// prose that only appears on a red build is prose nobody proofreads.
type pageClaims struct {
	// region is the region `has`/`absent` address; "" when none given.
	region string
	// menuHas is always about the menu region, whatever `region` says.
	menuHas []string
	has     []string
	absent  []string
	// hasCard/cardAbsent are sugar for the kanban-card region.
	hasCard    []string
	cardAbsent []string
}

// readPageClaims parses the claim keys, refusing a scalar where a list belongs
// for the same reason claimList does: a bare string reads as an empty list,
// which asserts the opposite of what the author wrote.
func readPageClaims(ls *lua.LState, tbl *lua.LTable) (pageClaims, error) {
	c := pageClaims{region: fieldString(ls, tbl, "region")}
	for _, f := range []struct {
		key string
		dst *[]string
	}{
		{"menu_has", &c.menuHas},
		{"has", &c.has},
		{"absent", &c.absent},
		{"has_card", &c.hasCard},
		{"card_absent", &c.cardAbsent},
	} {
		v, err := claimList(tbl, f.key)
		if err != nil {
			return c, err
		}
		*f.dst = v
	}
	return c, nil
}

// absentTexts collects every negative claim, whichever region it was made in.
//
// The negatives are the ones worth captioning: a reader can see what IS on a
// screenshot, but nothing in an image conveys that a draft title was checked
// for and found missing.
func (c pageClaims) absentTexts() []string {
	out := make([]string, 0, len(c.absent)+len(c.cardAbsent))
	out = append(out, c.absent...)
	out = append(out, c.cardAbsent...)
	return out
}

// validate enforces the two structural rules: at least one claim, and a
// region= whenever a region-scoped claim is made.
func (c pageClaims) validate() error {
	if len(c.menuHas) == 0 && len(c.has) == 0 && len(c.absent) == 0 &&
		len(c.hasCard) == 0 && len(c.cardAbsent) == 0 {

		return errors.New("asserts nothing. Give at least one of menu_has=, has=, absent=, " +
			"has_card= or card_absent= — a call with no claim passes whatever the " +
			"page renders, which is worse than no call at all")
	}
	if c.region == "" && (len(c.has) > 0 || len(c.absent) > 0) {
		return fmt.Errorf("has=/absent= need a region= naming what they are about "+
			"(known regions: %s). Without one the claim has no subject",
			strings.Join(regionNames(), ", "))
	}
	if c.region != "" && len(c.has) == 0 && len(c.absent) == 0 {
		return fmt.Errorf("region=%q is given but nothing is claimed about it — "+
			"add has= or absent=", c.region)
	}
	return nil
}

// regionsUsed lists the distinct regions this call must fetch, in a stable
// order so a build's browser trips are deterministic.
func (c pageClaims) regionsUsed() []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if len(c.menuHas) > 0 {
		add("menu")
	}
	if len(c.hasCard) > 0 || len(c.cardAbsent) > 0 {
		add("kanban-card")
	}
	if len(c.has) > 0 || len(c.absent) > 0 {
		add(c.region)
	}
	return out
}

// check runs every claim against the fetched region texts and returns a
// human-readable failure, or "".
func (c pageClaims) check(view string, texts map[string][]string) string {
	var fails []string
	appendFail := func(region string, msg string) {
		fails = append(fails, fmt.Sprintf("  %s: %s\n%s", region, msg, dumpRegion(texts[region])))
	}

	if len(c.menuHas) > 0 {
		if msg := checkRegionText("menu", texts["menu"], c.menuHas, nil); msg != "" {
			appendFail("menu", msg)
		}
	}
	if len(c.hasCard) > 0 || len(c.cardAbsent) > 0 {
		if msg := checkRegionText("kanban-card", texts["kanban-card"], c.hasCard, c.cardAbsent); msg != "" {
			appendFail("kanban-card", msg)
		}
	}
	if len(c.has) > 0 || len(c.absent) > 0 {
		if msg := checkRegionText(c.region, texts[c.region], c.has, c.absent); msg != "" {
			appendFail(c.region, msg)
		}
	}

	if len(fails) == 0 {
		return ""
	}
	return fmt.Sprintf("page{view=%q} failed\n%s", view, strings.Join(fails, "\n"))
}

// checkRegionText is the matcher: case-sensitive substring over the region's
// accessible text.
//
// # Why substring and not regex
//
// A manual asserting a rendered label wants "the badge says en". Regex invites
// cleverness that outlives the person who wrote it, and — worse — a regex
// mis-anchored by one character silently stops matching, which for `absent=`
// means a silent pass. Substring has exactly one behavior.
//
// # Why a region that resolves to NO element is a failure
//
// An empty region satisfies every `absent=` claim, so a mistyped view argument
// or a renamed anchor would turn the negative claims — the ones this verb
// recommends most — vacuously green. "Asserted nothing" is the defect class
// this feature exists to kill, so it fails loudly instead.
func checkRegionText(region string, got, has, absent []string) string {
	if len(got) == 0 {
		return fmt.Sprintf("region %q matched no element on the page. An empty region would "+
			"satisfy every absent= claim, so it is a failure rather than a pass — check the "+
			"view/list/entity arguments and the `as` role's read access", region)
	}
	joined := strings.Join(got, "\n")

	var missing, unexpected []string
	for _, want := range has {
		if !strings.Contains(joined, want) {
			missing = append(missing, want)
		}
	}
	for _, no := range absent {
		if strings.Contains(joined, no) {
			unexpected = append(unexpected, no)
		}
	}
	if len(missing) == 0 && len(unexpected) == 0 {
		return ""
	}
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing "+strings.Join(quoteAll(dedupe(missing)), ", "))
	}
	if len(unexpected) > 0 {
		parts = append(parts, "unexpectedly present "+strings.Join(quoteAll(dedupe(unexpected)), ", "))
	}
	return strings.Join(parts, "; ")
}

// Caps on the failure dump. A failure must print WHAT WAS THERE, not just what
// was missing — seeing `site-nl` where `en` was expected is the entire
// diagnosis for TKT-PI17Z6. But a full-page region can be enormous, so the dump
// is bounded: enough to read a label, not enough to bury the message.
const (
	maxDumpItems = 40
	maxDumpBytes = 2048
)

// dumpRegion renders the region's text content for a failure message, capped.
func dumpRegion(got []string) string {
	if len(got) == 0 {
		return "    (region matched no element)"
	}
	var b strings.Builder
	b.WriteString("    found:\n")
	used := 0
	for i, s := range got {
		if i >= maxDumpItems {
			fmt.Fprintf(&b, "      … and %d more\n", len(got)-i)
			break
		}
		line := "      - " + s + "\n"
		if used+len(line) > maxDumpBytes {
			fmt.Fprintf(&b, "      … and %d more (output capped)\n", len(got)-i)
			break
		}
		b.WriteString(line)
		used += len(line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
