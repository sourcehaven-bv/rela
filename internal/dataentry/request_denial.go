package dataentry

import (
	"errors"
	"net/http"
)

// denialError is a refusal from a gate chain that runs on more than one
// transport (TKT-1TS55R). The HTTP routes write it as the problem response
// they always gave; rela.attach_render hands the script its title.
type denialError struct {
	status int
	code   string
	title  string
}

// deny builds a [*denialError].
func deny(status int, code, title string) *denialError {
	return &denialError{status: status, code: code, title: title}
}

// Error is the title. It is written for the caller, so a script may show
// it.
func (d *denialError) Error() string {
	return d.title
}

// writeRequestDenial answers err from a gate chain: a [*denialError] as its
// own response, anything else as a gate error.
func writeRequestDenial(w http.ResponseWriter, r *http.Request, err error) {
	var d *denialError
	if errors.As(err, &d) {
		writeV1Error(w, r, d.status, d.code, d.title, "")
		return
	}
	writeGateError(w, r, err)
}
