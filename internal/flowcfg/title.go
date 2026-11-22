package flowcfg

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// MaxTitleLen caps a run title, and MaxFailedMessageLen a flow's
// `failed_message:`, in characters.
const (
	MaxTitleLen         = 200
	MaxFailedMessageLen = 500
)

// runTitle is the flow's `title:` template (BUG-53GP5F5); set says the
// flow declares one.
type runTitle struct {
	tpl Template
	set bool
}

// title compiles the flow's `title:`. A template reads what an output
// template reads, and is held to the same visible: rule: the title is
// stored on the run, which every reader of the subject reads.
func (l *loader) title(key, src string, f *Flow) runTitle {
	if src == "" {
		return runTitle{}
	}
	if n := utf8.RuneCountInString(src); n > MaxTitleLen {
		l.add(key, fmt.Sprintf("is %d characters; at most %d", n, MaxTitleLen))
		return runTitle{}
	}
	tpl, reason := l.compileTemplate("title", src, metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
		f.subject)
	if reason != "" {
		l.add(key, reason)
		return runTitle{}
	}
	if slices.Contains(tpl.Refs(), RefRunID) {
		l.add(key, "may not read {{run.id}}: the title is written before the run has an id")
		return runTitle{}
	}
	return runTitle{tpl: tpl, set: true}
}

// failedMessage checks the flow's `failed_message:`.
func (l *loader) failedMessage(key, msg string) string {
	if n := utf8.RuneCountInString(msg); n > MaxFailedMessageLen {
		l.add(key, fmt.Sprintf("is %d characters; at most %d", n, MaxFailedMessageLen))
		return ""
	}
	return strings.TrimSpace(msg)
}

// RunTitle is the title the engine stores on a run of f on subj when the
// run starts: the `title:` template, or the flow's label. The default
// copies nothing of the subject: a reader's view of the run adds the
// subject's title as that reader may read it, and a stored copy would
// outlive the subject. A later change of the subject does not change a
// stored title.
//
// Nil: subj accepted — the template without subject values.
func (f *Flow) RunTitle(subj *entity.Entity, now time.Time) string {
	var title string
	if f.title.set {
		vars := Vars{Now: now}
		if subj != nil {
			vars.Subject, vars.Props = subj.Ref(), subj.Properties
		}
		if v, ok := f.title.tpl.Eval(vars); ok && v != nil {
			title = strings.Join(strings.Fields(fmt.Sprint(v)), " ")
		}
	}
	if title == "" {
		title = f.label
	}
	if utf8.RuneCountInString(title) > MaxTitleLen {
		r := []rune(title)
		title = string(r[:MaxTitleLen-1]) + "…"
	}
	return title
}

// HasTitle reports that f declares `title:`.
func (f *Flow) HasTitle() bool { return f.title.set }

// FailedMessage is `failed_message:`: what a reader who does not manage a
// run reads while its job step failed, instead of rela's default. Empty
// when the flow sets none.
func (f *Flow) FailedMessage() string { return f.failedMessage }
