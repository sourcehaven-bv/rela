package remotesync

// Report says what one run did, or in a dry run would do.
type Report struct {
	Remote string `json:"remote"`
	RunID  string `json:"run_id,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
	// Refused says why the gone guard refused the run; nothing was applied.
	Refused string `json:"refused,omitempty"`
	// Gone is how many linked rows the source no longer lists.
	Gone int `json:"gone"`
	// Waiting is how many changed rows were not fetched because the run
	// reached max_fetch or max_bytes. They are fetched by a later run.
	Waiting   int    `json:"waiting,omitempty"`
	Entities  Counts `json:"entities"`
	Relations Counts `json:"relations"`
	// Pushed counts what a remote with push sent to the remote.
	Pushed *PushCounts `json:"pushed,omitempty"`
	// Held is how many entries the remote's conflict list holds after the
	// run. `rela remote conflicts` lists them.
	Held int `json:"held"`
	// Skipped lists the source relation types the source refused to
	// compare. Their rows are left alone.
	Skipped []string `json:"skipped,omitempty"`
	// Issues lists the rows that were not applied, and why.
	Issues []Issue `json:"issues,omitempty"`
}

// Counts are the outcomes of one kind of row.
type Counts struct {
	Created   int `json:"created"`
	Adopted   int `json:"adopted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	// Conflicts are rows where both sides changed an owned field; the row
	// is left as it is and compared again on the next run.
	Conflicts int `json:"conflicts"`
	// Rejected rows did not fit the local schema.
	Rejected int `json:"rejected"`
	// Dropped rows were deleted locally and are not recreated.
	Dropped int `json:"dropped"`
	// Deferred rows wait for the next run: over max_fetch or max_bytes, an
	// endpoint not pulled yet, or a row that went away during the run.
	Deferred int `json:"deferred"`
	Detached int `json:"detached"`
	Deleted  int `json:"deleted"`
	// Failed rows hit an error that may pass; they are tried again.
	Failed int `json:"failed"`
}

// PushCounts are the outcomes of the push of a remote with push.
type PushCounts struct {
	// Created rows are new local rows the remote now has too.
	Created int `json:"created"`
	// Updated rows had local changes the remote accepted.
	Updated int `json:"updated"`
	// Conflicts are rows the remote changed again during the push, twice;
	// they are compared again on the next run.
	Conflicts int `json:"conflicts"`
	// Refused rows were refused by the remote and are held on the conflict
	// list.
	Refused int `json:"refused"`
	// Deferred rows wait for the next run: over max_fetch or max_bytes, or
	// gone from the remote during the run.
	Deferred int `json:"deferred"`
	// Failed rows hit an error that may pass; they are tried again.
	Failed int `json:"failed"`
}

// Issue is one row that was not applied as the source has it.
type Issue struct {
	Kind    string `json:"kind"`
	Address string `json:"address"`
	// Local is the local entity id or relation key, when there is one.
	Local  string `json:"local,omitempty"`
	Reason string `json:"reason"`
	// Fields names the fields in conflict.
	Fields []string `json:"fields,omitempty"`
}

// Issue kinds.
const (
	IssueConflict = "conflict"
	IssueRejected = "rejected"
	IssueKept     = "kept"
	IssueFailed   = "failed"
	IssueRefused  = "refused"
)
