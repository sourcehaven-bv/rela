// Package remotestatus holds the values a remote's status page shows: its
// settings, the types it maps, how its runs went, and what the latest rule
// decided for a field (TKT-0FMYPEZ).
//
// It is a leaf of plain values with JSON tags and imports only the
// standard library, so remotesync, the scheduler and the data-entry routes
// share one shape without the routes importing remotesync, which reaches
// the store. The values hold no credential and no remote error text.
package remotestatus

import (
	"encoding/json"
	"time"
)

// Remote is one remote's settings, mapped types and run health.
type Remote struct {
	Name     string
	Settings Settings
	// CredentialKind is the kind of source the credential is read from:
	// env, file or secret. It is never the source's name or value.
	CredentialKind string
	// MaxClockSkew and SupersededRetention are the parsed durations, with
	// their defaults applied.
	MaxClockSkew        time.Duration
	SupersededRetention time.Duration
	Types               []MappedType
	Run                 Run
}

// Settings are the settings a remote declares in remotes.yaml that its
// status shows. remotesync reads them as part of a remote; a status holds
// them with their defaults applied. None of them is a credential.
type Settings struct {
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// URL is the source's base address, as for `rela reconcile --remote`.
	URL string `yaml:"url" json:"url"`
	// RunAs is the identity a scheduled pull runs and writes as. Empty means
	// the scheduler's own identity, as for a scheduled task. A pull by hand
	// runs as the person who starts it.
	RunAs string `yaml:"run_as,omitempty" json:"run_as,omitempty"`
	// Every is the pull schedule in schedules.yaml syntax. Empty means the
	// remote is pulled only by `rela remote pull`.
	Every string `yaml:"every,omitempty" json:"every,omitempty"`
	// Push makes the remote two-way: after each pull, the run pushes the
	// local changes to mapped fields, and the shared local rows the remote
	// does not have yet, through the remote's write API (DEC-2NFLAHB).
	Push bool `yaml:"push,omitempty" json:"push"`
	// Conflict is the rule for a field both sides changed since the last
	// sync: hold, latest, local or remote.
	Conflict string `yaml:"conflict,omitempty" json:"conflict"`
	// OnGone is what a pull does to a linked row the source no longer
	// has: detach or delete.
	OnGone string `yaml:"on_gone,omitempty" json:"on_gone,omitempty"`
	// SendDeletes makes a push delete the remote row of a linked local row
	// that was deleted here. It needs Push.
	SendDeletes bool `yaml:"send_deletes,omitempty" json:"send_deletes"`
}

// MappedType is one type a remote maps: an entity type, or a relation type
// when Kind is "relation".
type MappedType struct {
	Kind   string        `json:"kind,omitempty"`
	Local  string        `json:"local"`
	Source string        `json:"source"`
	Fields []MappedField `json:"fields"`
}

// MappedField is one mapped field and its conflict rule.
type MappedField struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
	Rule   string `json:"rule"`
}

// Run is how a remote's scheduled pulls went.
type Run struct {
	// LastRun is when the last successful pull started; zero when none has.
	LastRun time.Time `json:"last_run,omitzero"`
	// NextRun is when the schedule next makes a pull due. It is zero when
	// the remote has no schedule, or a pull is due now.
	NextRun time.Time `json:"next_run,omitzero"`
	// Failures counts the pulls that failed in a row since the last
	// success, and NextRetry is when the next attempt is due.
	NextRetry time.Time `json:"next_retry,omitzero"`
	Failures  int       `json:"failures"`
	// Running reports a pull, resolve or other run holding the remote now.
	Running bool `json:"running"`
}

// Superseded is what the latest rule decided for a field: the side that
// won, both sides' last-updated times, and the losing value. A losing local
// value is kept here; a losing remote value is the entry's remote value.
// Either is left out when larger than the conflict list keeps.
type Superseded struct {
	Winner            string          `json:"winner"`
	LocalValue        json.RawMessage `json:"local_value,omitempty"`
	LocalValueOmitted bool            `json:"local_value_omitted,omitempty"`
	LocalUpdated      time.Time       `json:"local_updated"`
	RemoteUpdated     time.Time       `json:"remote_updated"`
}
