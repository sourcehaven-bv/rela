package docs

import (
	"fmt"
	"strings"
)

// Evidence rendering: what an assertion LEAVES BEHIND in the rendered manual.
//
// # Why an assertion renders at all
//
// An assertion used to emit nothing. It ran, it passed, and it vanished from
// the output — so a manual that said
//
//	The reader's world holds only what has been published:
//	<nothing>
//	That `absent` is the whole feature in one line.
//
// shipped prose pointing at evidence the reader could not see. The second
// sentence referred to a line that was not there. For a reader who cannot open
// the source that is worse than having no assertion: the claim is now
// unsupported AND the paragraph is incoherent.
//
// The audience for a rendered manual is not the person who wrote the island.
// It is an auditor, a new operator, or a reviewer signing off a publication
// rule. They need to see WHICH entities a world showed and WHO was refused,
// stated as a small table, not as a green CI tick they never look at.
//
// # Why the default is to render, and `emit=false` is the opt-out
//
// Forgetting `emit=true` under an opt-in default gives a silently invisible
// assertion — exactly the defect above, reintroduced one call at a time.
// Forgetting `emit=false` under an opt-out default gives an extra table in an
// end-user document, which a reviewer sees at a glance and deletes.
//
// Both mistakes stay visible, but only the opt-out default is also the right
// answer when nobody thinks about it. `emit=false` exists for genuine end-user
// documentation that must be tested without carrying the apparatus.
//
// # Why a passing claim renders and a failing one does not
//
// A failing assertion fails the BUILD, so there is no rendered document to put
// it in. Everything here therefore describes a claim that held.

// evidence is one rendered assertion block: a claim sentence and an optional
// table of what was observed.
type evidence struct {
	// claim is the human sentence — "In the published world, policy shows
	// exactly POL-1". Rendered as the block's lead line.
	claim string
	// header/rows are an optional observation table. Empty rows render the
	// claim alone, which suits a verb whose observation IS the claim (a
	// refused write has nothing further to show).
	header []string
	rows   [][]string
	// note is an optional trailing line for a qualification the claim needs
	// (e.g. that a denied read is indistinguishable from a missing one).
	note string
}

// render turns evidence into the markdown spliced into the manual.
//
// The check mark is the whole point for a non-technical reader: it says a
// machine confirmed this sentence during the build, as opposed to an author
// having typed it. The wording is deliberately plain — "verified" rather than
// "assertion passed" — because the reader is being told a fact about the
// system, not about the test suite.
func (e evidence) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "> **✓ Verified** — %s\n", e.claim)
	if len(e.rows) > 0 && len(e.header) > 0 {
		b.WriteString(">\n> ")
		for _, h := range e.header {
			fmt.Fprintf(&b, "| %s ", mdCell(h))
		}
		b.WriteString("|\n> ")
		for range e.header {
			b.WriteString("| --- ")
		}
		b.WriteString("|\n")
		for _, r := range e.rows {
			b.WriteString("> ")
			for _, c := range r {
				fmt.Fprintf(&b, "| %s ", mdCell(c))
			}
			b.WriteString("|\n")
		}
	}
	if e.note != "" {
		fmt.Fprintf(&b, ">\n> %s\n", e.note)
	}
	b.WriteString("\n")
	return b.String()
}

// emitEvidence renders ev unless the island opted out with emit=false.
//
// Centralized so every verb spells the opt-out the same way and so the
// blockquote shape is defined once — a manual mixing two evidence styles reads
// as two documents.
func emitEvidence(emit func(string), show bool, ev evidence) {
	if !show {
		return
	}
	emit(ev.render())
}

// joinIDs renders an id list for a claim sentence, or "(none)" when empty.
//
// "(none)" is a real answer for a world that excludes everything, and the most
// interesting one a publication rule can give, so it must read as a result
// rather than as a missing value.
func joinIDs(ids []string) string {
	if len(ids) == 0 {
		return "(none)"
	}
	return strings.Join(ids, ", ")
}

// worldPhrase names the view a claim was made in, for a sentence.
//
// The default world is spelled out rather than left implicit: "the default
// world (every entity, at its default face)" tells a reader who has not read
// the concept chapter what they are looking at.
func worldPhrase(world string) string {
	if world == "" {
		return "the default world"
	}
	return fmt.Sprintf("the `%s` world", world)
}
