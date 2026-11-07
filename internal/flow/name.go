package flow

import "regexp"

// validName is the grammar of a flow, state, timer, reminder and event
// name in flows.yaml.
var validName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ValidName reports whether s is a valid flow, state, timer, reminder or
// event name: a lowercase letter, then up to 63 lowercase letters, digits,
// underscores or hyphens. The flows.yaml loader and the run store both
// check it, so a name one accepts the other stores.
func ValidName(s string) bool { return validName.MatchString(s) }

// RunID names one run of a flow: the id of its _flow-run entity and the
// key of its execution state in the run store. A run id is never renamed.
type RunID string
