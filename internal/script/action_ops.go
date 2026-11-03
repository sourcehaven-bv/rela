package script

import "strings"

// The transport-neutral vocabulary of the data-entry action operations
// (TKT-NW9QSN). The gates live in the data-entry package; a second transport
// (the remote MCP tools) speaks these types so it renders the HTTP route's
// verdicts instead of re-deriving them.

// ActionInfo describes one action a caller may run.
type ActionInfo struct {
	ID          string
	Label       string
	Description string

	// Confirm is true when the action asks for confirmation before it runs;
	// ConfirmText is the question, empty for a plain `confirm: true`.
	Confirm     bool
	ConfirmText string

	// EntityBound is true for an `available_on` action: it runs against the
	// entity it was listed for and needs that entity's address.
	EntityBound bool

	// RequestScoped is true when the action accepts a request body: its
	// `request:` block sets `body: true`.
	RequestScoped bool
}

// ActionRunInput is one action run's caller-supplied input.
type ActionRunInput struct {
	// Address is the entity the action runs against: `ID` or `ID@face`.
	// Required for an entity-bound action; optional context otherwise.
	Address string

	// Body is the request body for a request-scoped action. Nil sends none;
	// a non-nil Body for an action without a `request:` block is refused.
	Body map[string]any

	// Confirmed answers the action's confirmation question. An action that
	// asks one runs only when it is true.
	Confirmed bool
}

// ActionError is a refused or failed action run, in the vocabulary of the
// HTTP API: Code is the error code the route answers (`action_not_found`,
// `permission_required`, `script_error`, …), Title and Detail its texts.
type ActionError struct {
	Code          string
	Title         string
	Detail        string
	CorrelationID string
}

func (e *ActionError) Error() string {
	var b strings.Builder
	b.WriteString(e.Title)
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	if e.Code != "" {
		b.WriteString(" [")
		b.WriteString(e.Code)
		b.WriteString("]")
	}
	if e.CorrelationID != "" {
		b.WriteString(" (correlation ")
		b.WriteString(e.CorrelationID)
		b.WriteString(")")
	}
	return b.String()
}
