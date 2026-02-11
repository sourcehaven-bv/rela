package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/commands"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// --- Reference model ---

// searchState tracks what we can predict about the search screen.
// Fields that depend on autocomplete/search logic are synced from the SUT
// after each command, since modeling those fully would duplicate the implementation.
type searchState struct {
	query     string
	cursorPos int

	// Synced from SUT after each command (not predicted by reference model)
	resultCount     int
	resultIndex     int
	showSuggestions bool
	suggestionCount int
	suggestionIndex int
}

func (s *searchState) clone() *searchState {
	return &searchState{
		query:           s.query,
		cursorPos:       s.cursorPos,
		resultCount:     s.resultCount,
		resultIndex:     s.resultIndex,
		showSuggestions: s.showSuggestions,
		suggestionCount: s.suggestionCount,
		suggestionIndex: s.suggestionIndex,
	}
}

// --- System under test ---

type searchSUT struct {
	app    *App
	search *SearchModel
}

// createPropertyTestApp builds a test app with enough data to exercise
// search, autocomplete, and result navigation.
func createPropertyTestApp() *App {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "string", Required: false},
					"priority": {Type: "integer", Required: false},
				},
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string", Required: false},
				},
			},
			"solution": {
				Label:    "Solution",
				IDPrefix: "SOL-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string", Required: false},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{},
	}

	g := graph.New()

	entities := []*model.Entity{
		{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{
			"title": "Authentication system", "status": "draft", "priority": 5,
		}},
		{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{
			"title": "API gateway", "status": "published", "priority": 3,
		}},
		{ID: "REQ-003", Type: "requirement", Properties: map[string]interface{}{
			"title": "Logging framework", "status": "draft", "priority": 2,
		}},
		{ID: "DEC-001", Type: "decision", Properties: map[string]interface{}{
			"title": "Use OAuth 2.0", "status": "accepted",
		}},
		{ID: "DEC-002", Type: "decision", Properties: map[string]interface{}{
			"title": "REST API design", "status": "proposed",
		}},
		{ID: "SOL-001", Type: "solution", Properties: map[string]interface{}{
			"title": "OAuth implementation", "status": "draft",
		}},
	}

	for _, e := range entities {
		g.AddNode(e)
	}

	return &App{
		metamodel: meta,
		graph:     g,
		project:   &project.Context{Root: "/tmp/test"},
		width:     80,
		height:    40,
	}
}

func newSearchSUT() *searchSUT {
	app := createPropertyTestApp()
	app.searchVersionCounter++
	search := NewSearchModel(app)
	app.search = search
	app.screen = ScreenSearch
	return &searchSUT{app: app, search: search}
}

// --- Invariant checks (run after every command) ---

// checkSUTInvariants verifies structural invariants on the real SUT.
// Returns a PropResult indicating pass or failure with error details.
func checkSUTInvariants(sys *searchSUT, label string) *gopter.PropResult {
	search := sys.search

	// Invariant 1: cursor position always valid
	if search.cursorPos < 0 || search.cursorPos > len(search.query) {
		return &gopter.PropResult{
			Status: gopter.PropFalse,
			Error: fmt.Errorf("%s: cursor out of bounds: pos=%d, queryLen=%d, query=%q",
				label, search.cursorPos, len(search.query), search.query),
		}
	}

	// Invariant 2: resultIndex within bounds
	if len(search.results) > 0 && (search.resultIndex < 0 || search.resultIndex >= len(search.results)) {
		return &gopter.PropResult{
			Status: gopter.PropFalse,
			Error: fmt.Errorf("%s: resultIndex out of bounds: idx=%d, count=%d",
				label, search.resultIndex, len(search.results)),
		}
	}
	if len(search.results) == 0 && search.resultIndex != 0 {
		return &gopter.PropResult{
			Status: gopter.PropFalse,
			Error: fmt.Errorf("%s: resultIndex=%d with no results",
				label, search.resultIndex),
		}
	}

	// Invariant 3: suggestionIndex within bounds when shown
	if search.showSuggestions && len(search.suggestions) > 0 {
		if search.suggestionIndex < 0 || search.suggestionIndex >= len(search.suggestions) {
			return &gopter.PropResult{
				Status: gopter.PropFalse,
				Error: fmt.Errorf("%s: suggestionIndex out of bounds: idx=%d, count=%d",
					label, search.suggestionIndex, len(search.suggestions)),
			}
		}
	}

	// Invariant 4: showSuggestions consistency
	if search.showSuggestions && len(search.suggestions) == 0 {
		return &gopter.PropResult{
			Status: gopter.PropFalse,
			Error:  fmt.Errorf("%s: showSuggestions=true but suggestions is empty", label),
		}
	}

	// Note: we don't check "empty query => no results" because results from
	// a previous search can linger until the next async search completes.
	// This is expected transient state, not an invariant violation.

	// Invariant 6: View() must not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				panic(fmt.Sprintf("%s: View() panicked: %v (query=%q, cursor=%d)",
					label, r, search.query, search.cursorPos))
			}
		}()
		search.View(80, 40)
	}()

	return &gopter.PropResult{Status: gopter.PropTrue}
}

// syncStateFromSUT copies non-deterministic fields from the SUT into the reference state.
func syncStateFromSUT(s *searchState, sys *searchSUT) {
	search := sys.search
	s.resultCount = len(search.results)
	s.resultIndex = search.resultIndex
	s.showSuggestions = search.showSuggestions
	s.suggestionCount = len(search.suggestions)
	s.suggestionIndex = search.suggestionIndex
}

// --- Command implementations ---

// Each command: Run (execute on SUT), NextState (predict reference),
// PostCondition (check invariants + sync), PreCondition, String.

// postConditionWithInvariants is the shared postcondition that checks all
// invariants and syncs the reference model from the SUT.
func postConditionWithInvariants(label string) func(commands.State, commands.Result) *gopter.PropResult {
	return func(state commands.State, result commands.Result) *gopter.PropResult {
		s := state.(*searchState)
		sys := result.(*searchSUT)

		// Check structural invariants
		r := checkSUTInvariants(sys, label)
		if r.Status != gopter.PropTrue {
			return r
		}

		// Check reference model agreement (query & cursor)
		if sys.search.query != s.query {
			return &gopter.PropResult{
				Status: gopter.PropFalse,
				Error: fmt.Errorf("%s: query mismatch: real=%q, ref=%q",
					label, sys.search.query, s.query),
			}
		}
		if sys.search.cursorPos != s.cursorPos {
			return &gopter.PropResult{
				Status: gopter.PropFalse,
				Error: fmt.Errorf("%s: cursorPos mismatch: real=%d, ref=%d",
					label, sys.search.cursorPos, s.cursorPos),
			}
		}

		// Sync non-deterministic state
		syncStateFromSUT(s, sys)

		return &gopter.PropResult{Status: gopter.PropTrue}
	}
}

// typeCharCmd inserts a single character at the cursor
type typeCharCmd struct {
	char rune
}

func (c *typeCharCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c.char}})
	return sys
}

func (c *typeCharCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	newQuery := s.query[:s.cursorPos] + string(c.char) + s.query[s.cursorPos:]
	if len(newQuery) > maxQueryLength {
		return s
	}
	ns := s.clone()
	ns.query = newQuery
	ns.cursorPos = s.cursorPos + 1
	return ns
}

func (c *typeCharCmd) PreCondition(commands.State) bool { return true }

func (c *typeCharCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants(fmt.Sprintf("Type('%c')", c.char))(state, result)
}

func (c *typeCharCmd) String() string { return fmt.Sprintf("Type('%c')", c.char) }

// typeSpaceCmd inserts a space
type typeSpaceCmd struct{}

func (c *typeSpaceCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeySpace})
	return sys
}

func (c *typeSpaceCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	newQuery := s.query[:s.cursorPos] + " " + s.query[s.cursorPos:]
	if len(newQuery) > maxQueryLength {
		return s
	}
	ns := s.clone()
	ns.query = newQuery
	ns.cursorPos = s.cursorPos + 1
	return ns
}

func (c *typeSpaceCmd) PreCondition(commands.State) bool { return true }

func (c *typeSpaceCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("Space")(state, result)
}

func (c *typeSpaceCmd) String() string { return "Space" }

// typeStringCmd types a whole string character by character
type typeStringCmd struct {
	str string
}

func (c *typeStringCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	for _, ch := range c.str {
		if ch == ' ' {
			sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeySpace})
		} else {
			sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		}
	}
	return sys
}

func (c *typeStringCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	ns := s.clone()
	// Insert character by character, matching Run behavior.
	// Each character is checked against maxQueryLength independently.
	for _, ch := range c.str {
		charStr := string(ch)
		newQuery := ns.query[:ns.cursorPos] + charStr + ns.query[ns.cursorPos:]
		if len(newQuery) > maxQueryLength {
			break
		}
		ns.query = newQuery
		ns.cursorPos += len(charStr)
	}
	return ns
}

func (c *typeStringCmd) PreCondition(commands.State) bool { return true }

func (c *typeStringCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants(fmt.Sprintf("TypeString(%q)", c.str))(state, result)
}

func (c *typeStringCmd) String() string { return fmt.Sprintf("TypeString(%q)", c.str) }

// backspaceCmd deletes the character before the cursor
type backspaceCmd struct{}

func (c *backspaceCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyBackspace})
	return sys
}

func (c *backspaceCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	if s.cursorPos <= 0 || s.query == "" {
		return s
	}
	ns := s.clone()
	ns.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
	ns.cursorPos = s.cursorPos - 1
	return ns
}

func (c *backspaceCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.cursorPos > 0 && s.query != ""
}

func (c *backspaceCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("Backspace")(state, result)
}

func (c *backspaceCmd) String() string { return "Backspace" }

// cursorLeftCmd moves the cursor left
type cursorLeftCmd struct{}

func (c *cursorLeftCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyLeft})
	return sys
}

func (c *cursorLeftCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	if s.cursorPos <= 0 {
		return s
	}
	ns := s.clone()
	ns.cursorPos = s.cursorPos - 1
	return ns
}

func (c *cursorLeftCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.cursorPos > 0
}

func (c *cursorLeftCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("CursorLeft")(state, result)
}

func (c *cursorLeftCmd) String() string { return "CursorLeft" }

// cursorRightCmd moves the cursor right
type cursorRightCmd struct{}

func (c *cursorRightCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyRight})
	return sys
}

func (c *cursorRightCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	if s.cursorPos >= len(s.query) {
		return s
	}
	ns := s.clone()
	ns.cursorPos = s.cursorPos + 1
	return ns
}

func (c *cursorRightCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.cursorPos < len(s.query)
}

func (c *cursorRightCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("CursorRight")(state, result)
}

func (c *cursorRightCmd) String() string { return "CursorRight" }

// navigateDownCmd moves selection down in results or suggestions
type navigateDownCmd struct{}

func (c *navigateDownCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyDown})
	return sys
}

func (c *navigateDownCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	ns := s.clone()
	if s.showSuggestions {
		if s.suggestionIndex < s.suggestionCount-1 {
			ns.suggestionIndex++
		}
	} else if s.resultIndex < s.resultCount-1 {
		ns.resultIndex++
	}
	return ns
}

func (c *navigateDownCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	if s.showSuggestions {
		return s.suggestionCount > 1 && s.suggestionIndex < s.suggestionCount-1
	}
	return s.resultCount > 1 && s.resultIndex < s.resultCount-1
}

func (c *navigateDownCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("NavigateDown")(state, result)
}

func (c *navigateDownCmd) String() string { return "NavigateDown" }

// navigateUpCmd moves selection up in results or suggestions
type navigateUpCmd struct{}

func (c *navigateUpCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyUp})
	return sys
}

func (c *navigateUpCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	ns := s.clone()
	if s.showSuggestions {
		if s.suggestionIndex > 0 {
			ns.suggestionIndex--
		}
	} else if s.resultIndex > 0 {
		ns.resultIndex--
	}
	return ns
}

func (c *navigateUpCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	if s.showSuggestions {
		return s.suggestionIndex > 0
	}
	return s.resultIndex > 0
}

func (c *navigateUpCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("NavigateUp")(state, result)
}

func (c *navigateUpCmd) String() string { return "NavigateUp" }

// clearCmd clears the query with ctrl+u
type clearCmd struct{}

func (c *clearCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyCtrlU})
	return sys
}

func (c *clearCmd) NextState(_ commands.State) commands.State {
	return &searchState{}
}

func (c *clearCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.query != ""
}

func (c *clearCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	return postConditionWithInvariants("Clear")(state, result)
}

func (c *clearCmd) String() string { return "Clear(Ctrl+U)" }

// tabCmd accepts the current autocomplete suggestion
type tabCmd struct{}

func (c *tabCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	sys.search.Update(sys.app, tea.KeyMsg{Type: tea.KeyTab})
	return sys
}

func (c *tabCmd) NextState(state commands.State) commands.State {
	// Tab modifies the query in complex ways (token replacement).
	// We can't predict the new query, so we mark it for sync.
	// PostCondition will update query/cursorPos from SUT.
	s := state.(*searchState)
	ns := s.clone()
	ns.showSuggestions = false
	ns.suggestionCount = 0
	ns.suggestionIndex = 0
	return ns
}

func (c *tabCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.showSuggestions && s.suggestionCount > 0
}

func (c *tabCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	sys := result.(*searchSUT)
	s := state.(*searchState)

	// Check invariants first
	r := checkSUTInvariants(sys, "Tab")
	if r.Status != gopter.PropTrue {
		return r
	}

	// Tab modifies query unpredictably, so sync from SUT
	s.query = sys.search.query
	s.cursorPos = sys.search.cursorPos
	syncStateFromSUT(s, sys)

	return &gopter.PropResult{Status: gopter.PropTrue}
}

func (c *tabCmd) String() string { return "Tab" }

// syncSearchCmd executes the pending search synchronously,
// simulating the debounce completing.
type syncSearchCmd struct{}

func (c *syncSearchCmd) Run(sut commands.SystemUnderTest) commands.Result {
	sys := sut.(*searchSUT)
	search := sys.search

	if search.query != "" {
		results, errs := search.performSearch(sys.app, search.query)
		search.HandleSearchResults(searchResultsMsg{
			results:     results,
			query:       search.query,
			version:     search.searchVersion,
			baseVersion: search.baseVersion,
			errors:      errs,
		})
	}
	return sys
}

func (c *syncSearchCmd) NextState(state commands.State) commands.State {
	s := state.(*searchState)
	ns := s.clone()
	ns.resultIndex = 0 // reset on new results
	return ns
}

func (c *syncSearchCmd) PreCondition(state commands.State) bool {
	s := state.(*searchState)
	return s.query != ""
}

func (c *syncSearchCmd) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	sys := result.(*searchSUT)
	s := state.(*searchState)

	r := checkSUTInvariants(sys, "SyncSearch")
	if r.Status != gopter.PropTrue {
		return r
	}

	syncStateFromSUT(s, sys)
	return &gopter.PropResult{Status: gopter.PropTrue}
}

func (c *syncSearchCmd) String() string { return "SyncSearch" }

// --- Stateful test configuration ---

var searchCmdsProto = &commands.ProtoCommands{
	NewSystemUnderTestFunc: func(commands.State) commands.SystemUnderTest {
		return newSearchSUT()
	},
	InitialStateGen: gen.IntRange(0, 0).Map(func(int) *searchState {
		return &searchState{}
	}),
	InitialPreConditionFunc: func(commands.State) bool { return true },
	GenCommandFunc: func(state commands.State) gopter.Gen {
		s := state.(*searchState)

		// Always available commands
		cmds := []gopter.Gen{
			gen.AlphaChar().Map(func(c rune) commands.Command { return &typeCharCmd{char: c} }),
			gen.Const(&typeSpaceCmd{}),
		}

		// Cursor movement (only when valid)
		if s.cursorPos > 0 {
			cmds = append(cmds, gen.Const(&cursorLeftCmd{}))
		}
		if s.cursorPos < len(s.query) {
			cmds = append(cmds, gen.Const(&cursorRightCmd{}))
		}

		// Backspace
		if s.cursorPos > 0 && s.query != "" {
			cmds = append(cmds, gen.Const(&backspaceCmd{}))
		}

		// Navigation (only when there's something to navigate)
		if s.showSuggestions {
			if s.suggestionCount > 1 && s.suggestionIndex < s.suggestionCount-1 {
				cmds = append(cmds, gen.Const(&navigateDownCmd{}))
			}
			if s.suggestionIndex > 0 {
				cmds = append(cmds, gen.Const(&navigateUpCmd{}))
			}
			if s.suggestionCount > 0 {
				cmds = append(cmds, gen.Const(&tabCmd{}))
			}
		} else {
			if s.resultCount > 1 && s.resultIndex < s.resultCount-1 {
				cmds = append(cmds, gen.Const(&navigateDownCmd{}))
			}
			if s.resultIndex > 0 {
				cmds = append(cmds, gen.Const(&navigateUpCmd{}))
			}
		}

		// Clear
		if s.query != "" {
			cmds = append(cmds, gen.Const(&clearCmd{}))
		}

		// Sync search (to populate results/suggestions for navigation)
		if s.query != "" {
			cmds = append(cmds, gen.Const(&syncSearchCmd{}))
		}

		// Composable property filter: prop:<name><op><value>
		cmds = append(cmds, genComposedFilter().Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		// Composable type filter: type:<entity>, type:<prefix>, type:<a>,<b>
		cmds = append(cmds, genComposedTypeFilter().Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		// Composable status filter: status:<value>, status:<prefix>
		cmds = append(cmds, genComposedStatusFilter().Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		// Free text words from entity content
		cmds = append(cmds, gen.OneConstOf(
			freeTextWords[0], freeTextWords[1], freeTextWords[2],
			freeTextWords[3], freeTextWords[4], freeTextWords[5],
			freeTextWords[6], freeTextWords[7], freeTextWords[8],
		).Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		// Entity IDs
		cmds = append(cmds, gen.OneConstOf(
			entityIDs[0], entityIDs[1], entityIDs[2],
			entityIDs[3], entityIDs[4], entityIDs[5],
		).Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		// Quoted phrases
		cmds = append(cmds, gen.OneConstOf(
			quotedPhrases[0], quotedPhrases[1],
			quotedPhrases[2], quotedPhrases[3],
		).Map(func(s string) commands.Command {
			return &typeStringCmd{str: s}
		}))

		return gen.OneGenOf(cmds...)
	},
}

// --- Shared generators ---

// stringToKeyMsgs converts a string into a slice of tea.KeyMsg,
// handling spaces as KeySpace and everything else as KeyRunes.
func stringToKeyMsgs(s string) []tea.KeyMsg {
	msgs := make([]tea.KeyMsg, 0, len(s))
	for _, ch := range s {
		if ch == ' ' {
			msgs = append(msgs, tea.KeyMsg{Type: tea.KeySpace})
		} else {
			msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		}
	}
	return msgs
}

// --- Building blocks for composable filter generation ---
// These slices are drawn from the test data in createPropertyTestApp
// and are mixed/matched by generators to produce novel combinations.

var (
	entityTypes    = []string{"requirement", "decision", "solution"}
	typePrefixes   = []string{"req", "dec", "sol", "re", "d", "s"} // partial prefix matches
	propertyNames  = []string{"status", "priority", "title"}
	operators      = []string{"=", "!=", "<", "<=", ">", ">=", "=~"}
	statusValues   = []string{"draft", "published", "accepted", "proposed"}
	priorityValues = []string{"1", "2", "3", "4", "5", "0", "10", "-1", "999"}
	freeTextWords  = []string{
		"Authentication", "OAuth", "API", "REST", "Logging",
		"gateway", "framework", "implementation", "design",
	}
	entityIDs = []string{
		"REQ-001", "REQ-002", "REQ-003", "DEC-001", "DEC-002", "SOL-001",
	}
	quotedPhrases = []string{
		`"OAuth 2.0"`, `"REST API"`, `"API gateway"`, `"Logging framework"`,
	}
	// Glob/regex patterns for =~ operator
	globPatterns = []string{"*auth*", "*API*", "draft*", "*tion", "pub*", "*"}
)

// genComposedFilter generates a property filter by randomly combining
// prefix + property name + operator + value, producing combinations
// like "prop:priority>=draft" that no human would write.
func genComposedFilter() gopter.Gen {
	propName := gen.OneConstOf(propertyNames[0], propertyNames[1], propertyNames[2])
	op := gen.OneConstOf(operators[0], operators[1], operators[2], operators[3], operators[4], operators[5], operators[6])

	// Mix status values, priority values, and glob patterns as the value
	allValues := make([]string, 0, len(statusValues)+len(priorityValues)+len(globPatterns))
	allValues = append(allValues, statusValues...)
	allValues = append(allValues, priorityValues...)
	allValues = append(allValues, globPatterns...)
	val := gen.OneConstOf(
		allValues[0], allValues[1], allValues[2], allValues[3],
		allValues[4], allValues[5], allValues[6], allValues[7],
		allValues[8], allValues[9], allValues[10], allValues[11],
		allValues[12], allValues[13], allValues[14], allValues[15],
		allValues[16], allValues[17], allValues[18],
	)

	return gopter.CombineGens(propName, op, val).Map(func(vals []interface{}) string {
		return "prop:" + vals[0].(string) + vals[1].(string) + vals[2].(string) + " "
	})
}

// genComposedTypeFilter generates type filters with random type names,
// prefixes, and multi-type combinations.
func genComposedTypeFilter() gopter.Gen {
	return gen.OneGenOf(
		// Single type
		gen.OneConstOf(entityTypes[0], entityTypes[1], entityTypes[2]).Map(func(t string) string {
			return "type:" + t + " "
		}),
		// Prefix match
		gen.OneConstOf(typePrefixes[0], typePrefixes[1], typePrefixes[2],
			typePrefixes[3], typePrefixes[4], typePrefixes[5]).Map(func(p string) string {
			return "type:" + p
		}),
		// Multi-type (two random types joined by comma)
		gopter.CombineGens(
			gen.IntRange(0, len(entityTypes)-1),
			gen.IntRange(0, len(entityTypes)-1),
		).Map(func(vals []interface{}) string {
			return "type:" + entityTypes[vals[0].(int)] + "," + entityTypes[vals[1].(int)] + " "
		}),
		// Just the prefix (triggers autocomplete)
		gen.Const("type:"),
	)
}

// genComposedStatusFilter generates status: shortcut filters.
func genComposedStatusFilter() gopter.Gen {
	return gen.OneGenOf(
		gen.OneConstOf(statusValues[0], statusValues[1], statusValues[2], statusValues[3]).Map(func(v string) string {
			return "status:" + v + " "
		}),
		// Partial value (triggers autocomplete)
		gen.OneConstOf("d", "p", "a", "dr", "pub", "acc", "pro").Map(func(p string) string {
			return "status:" + p
		}),
		gen.Const("status:"),
	)
}

// genRealisticKeyMsgs generates a key message sequence that either types
// a composed search fragment or sends a single random/control key.
func genRealisticKeyMsgs() gopter.Gen {
	return gen.OneGenOf(
		// Single random character
		gen.AlphaChar().Map(func(c rune) []tea.KeyMsg {
			return []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{c}}}
		}),
		// Special operator characters
		gen.OneConstOf(':', '=', '"', '!', '<', '>', '~', '*', ',').Map(func(c rune) []tea.KeyMsg {
			return []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{c}}}
		}),
		// Control/navigation keys
		gen.OneConstOf(
			tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight,
			tea.KeyBackspace, tea.KeyTab, tea.KeySpace, tea.KeyEnter, tea.KeyCtrlU,
		).Map(func(kt tea.KeyType) []tea.KeyMsg {
			return []tea.KeyMsg{{Type: kt}}
		}),
		// Composed property filter (e.g., "prop:priority>=draft ")
		genComposedFilter().Map(stringToKeyMsgs),
		// Composed type filter
		genComposedTypeFilter().Map(stringToKeyMsgs),
		// Composed status filter
		genComposedStatusFilter().Map(stringToKeyMsgs),
		// Free text words
		gen.OneConstOf(freeTextWords[0], freeTextWords[1], freeTextWords[2],
			freeTextWords[3], freeTextWords[4], freeTextWords[5],
			freeTextWords[6], freeTextWords[7], freeTextWords[8]).Map(stringToKeyMsgs),
		// Entity IDs
		gen.OneConstOf(entityIDs[0], entityIDs[1], entityIDs[2],
			entityIDs[3], entityIDs[4], entityIDs[5]).Map(stringToKeyMsgs),
		// Quoted phrases
		gen.OneConstOf(quotedPhrases[0], quotedPhrases[1],
			quotedPhrases[2], quotedPhrases[3]).Map(stringToKeyMsgs),
	)
}

// flattenKeyMsgs flattens a slice of key message slices into a single slice.
func flattenKeyMsgs(groups [][]tea.KeyMsg) []tea.KeyMsg {
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	result := make([]tea.KeyMsg, 0, total)
	for _, g := range groups {
		result = append(result, g...)
	}
	return result
}

// --- Test functions ---

// TestSearchPropertyStateful uses gopter's stateful command testing
// with automatic shrinking of failing command sequences.
func TestSearchPropertyStateful(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200
	parameters.MaxSize = 40

	properties := gopter.NewProperties(parameters)
	properties.Property("search screen invariants hold under random commands",
		commands.Prop(searchCmdsProto))
	properties.TestingRun(t)
}

// TestSearchPropertyCrashFuzzing generates random key sequences including
// realistic search fragments and checks that nothing panics.
func TestSearchPropertyCrashFuzzing(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 500

	properties := gopter.NewProperties(parameters)

	properties.Property("search screen never panics under random keys", prop.ForAll(
		func(groups [][]tea.KeyMsg) bool {
			keys := flattenKeyMsgs(groups)
			sys := newSearchSUT()

			for _, key := range keys {
				// Skip enter when it would push detail screen
				if key.Type == tea.KeyEnter && len(sys.search.results) > 0 && !sys.search.showSuggestions {
					continue
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("panic on key %v with query=%q cursor=%d: %v",
								key, sys.search.query, sys.search.cursorPos, r)
						}
					}()
					sys.search.Update(sys.app, key)
				}()

				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("View() panic with query=%q cursor=%d: %v",
								sys.search.query, sys.search.cursorPos, r)
						}
					}()
					sys.search.View(80, 40)
				}()

				if sys.search.cursorPos < 0 || sys.search.cursorPos > len(sys.search.query) {
					t.Errorf("cursor out of bounds: pos=%d, queryLen=%d after key %v",
						sys.search.cursorPos, len(sys.search.query), key)
					return false
				}
			}

			return true
		},
		gen.SliceOfN(20, genRealisticKeyMsgs()),
	))

	properties.TestingRun(t)
}

// TestSearchPropertyWithSync tests with search execution interleaved
// between keystrokes, exercising result navigation and index bounds
// with realistic search queries.
func TestSearchPropertyWithSync(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 300

	properties := gopter.NewProperties(parameters)

	properties.Property("search with sync maintains invariants", prop.ForAll(
		func(groups [][]tea.KeyMsg) bool {
			keys := flattenKeyMsgs(groups)
			sys := newSearchSUT()

			for i, key := range keys {
				if key.Type == tea.KeyEnter && len(sys.search.results) > 0 && !sys.search.showSuggestions {
					continue
				}

				sys.search.Update(sys.app, key)

				// Execute search every 5th key to populate results
				if i%5 == 4 && sys.search.query != "" {
					results, errs := sys.search.performSearch(sys.app, sys.search.query)
					sys.search.HandleSearchResults(searchResultsMsg{
						results:     results,
						query:       sys.search.query,
						version:     sys.search.searchVersion,
						baseVersion: sys.search.baseVersion,
						errors:      errs,
					})
				}

				// Check all invariants
				if sys.search.cursorPos < 0 || sys.search.cursorPos > len(sys.search.query) {
					t.Errorf("step %d: cursor out of bounds: pos=%d, queryLen=%d",
						i, sys.search.cursorPos, len(sys.search.query))
					return false
				}

				if len(sys.search.results) > 0 && sys.search.resultIndex >= len(sys.search.results) {
					t.Errorf("step %d: resultIndex out of bounds: idx=%d, count=%d",
						i, sys.search.resultIndex, len(sys.search.results))
					return false
				}

				if sys.search.showSuggestions && len(sys.search.suggestions) > 0 &&
					sys.search.suggestionIndex >= len(sys.search.suggestions) {

					t.Errorf("step %d: suggestionIndex out of bounds: idx=%d, count=%d",
						i, sys.search.suggestionIndex, len(sys.search.suggestions))
					return false
				}

				sys.search.View(80, 40)
			}

			return true
		},
		gen.SliceOfN(25, genRealisticKeyMsgs()),
	))

	properties.TestingRun(t)
}
