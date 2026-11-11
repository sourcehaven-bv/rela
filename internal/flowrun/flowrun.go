package flowrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
)

// Limits every backend enforces.
const (
	// MaxPage caps the runs one list call returns.
	MaxPage = 500
	// MaxRunsPerSubject caps the stored runs of one flow on one subject
	// id, across its faces, so a start trigger that fires on every write
	// cannot grow one subject's runs without bound.
	MaxRunsPerSubject = 100
	// MaxDefinitionBytes caps a definition snapshot: the size cap of
	// flows.yaml itself.
	MaxDefinitionBytes = 1 << 20
	// MaxIDLen caps a run id and a subject id.
	MaxIDLen = 255
	// VersionLen is the length of a flow version: the hex prefix of the
	// definition hash the flows.yaml loader computes.
	VersionLen = 16
)

// Errors. Backends return them wrapped; test with errors.Is.
var (
	// ErrNotFound: no run, or no definition, under the key.
	ErrNotFound = errors.New("flowrun: not found")
	// ErrExists: a run with the id is stored already.
	ErrExists = errors.New("flowrun: run exists")
	// ErrSubjectBusy: a singleton run of the flow is stored on the subject.
	ErrSubjectBusy = errors.New("flowrun: subject has a singleton run of the flow")
	// ErrSubjectFull: the subject holds [MaxRunsPerSubject] runs of the flow.
	ErrSubjectFull = errors.New("flowrun: subject holds too many runs of the flow")
	// ErrConflict: the run's rev is not the one the caller read.
	ErrConflict = errors.New("flowrun: run changed since it was read")
	// ErrDefinitionConflict: the version is stored with another definition.
	ErrDefinitionConflict = errors.New("flowrun: version stored with another definition")
	// ErrInvalid: an argument fails its check.
	ErrInvalid = errors.New("flowrun: invalid argument")
	// ErrFull: the backend holds as many runs as it can (kvflowrun only).
	ErrFull = errors.New("flowrun: store is full")
)

// RunID names one run ([flow.RunID]); the store keys the run's state by
// it.
type RunID = flow.RunID

// Run is one stored run.
type Run struct {
	ID      RunID
	Flow    string
	Version string
	// Subject is the entity the run carries, at the face it started on.
	Subject entity.Ref
	// Singleton marks a run of a per_subject: one flow. While it is
	// stored, no other singleton run of its flow starts on its subject.
	Singleton bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// Rev rises by one on every update; it is 1 after create. Update and
	// Delete compare it.
	Rev int64
	// State is the run's execution state. It is embedded, so a backend or
	// a caller that changes one field carries the others along.
	State
}

// MaxLog caps [State.Log]. A longer log keeps its first entry, the start,
// and the newest ones.
const MaxLog = 32

// Via names what moved a run into a state, for [Step.Via].
type Via string

// The causes of a move.
const (
	ViaStart  Via = "start"
	ViaDone   Via = "done"
	ViaChange Via = "change"
	ViaTimer  Via = "timer"
	// ViaJoin: the parallel branches continued as one run at the join.
	ViaJoin Via = "join"
)

// Step is one entry of a run's timeline: the run entered State at At.
// The log lives in the run's stored state, so a finished run, whose state
// is dropped, keeps only what its header records.
type Step struct {
	At    time.Time `json:"at"`
	State string    `json:"state"`
	// From is the state the run left; empty for the start.
	From string `json:"from,omitempty"`
	Via  Via    `json:"via"`
	// Timer is the timer that fired, for a move via timer.
	Timer string `json:"timer,omitempty"`
	// By is the principal who caused the move, when a person did.
	By string `json:"by,omitempty"`
	// Branch names the parallel branch that moved: its first state. Empty
	// for a move of the run itself.
	Branch string `json:"branch,omitempty"`
}

// MaxClaimBy caps the length of [Claim.By] and [Step.By].
const MaxClaimBy = 256

// Claim records that a person is completing the step of the run's state
// at Generation (RR-LGRBAP4). It is stored before the person's values are
// written, so a second completion of the same step is refused before it
// writes. The move out of the state clears it; a claim older than the
// runtime's claim lifetime is void.
type Claim struct {
	Generation int64     `json:"generation"`
	By         string    `json:"by"`
	At         time.Time `json:"at"`
}

// MaxPendingDone caps [Pending.Done]: more than any state declares
// outputs.
const MaxPendingDone = 64

// MaxPendingFailures caps [Pending.Failures].
const MaxPendingFailures = 64

// Pending marks a state the run has entered but not settled in
// (RR-U30RNUZ): its outputs, its `done` branches or, in a final state, the
// end of the run are still to do. The runtime stores it with the move into
// the state, in the same write, and clears it once the run waits. A run
// stored with a mark after a crash is settled by whoever advances it next,
// so a move never loses its side effects. Outputs before Done are not
// repeated.
type Pending struct {
	// State is the state the run has not settled in.
	State string `json:"state"`
	// Generation is the instance generation that entered State. A mark
	// whose generation is not the instance's is stale and ignored.
	Generation int64 `json:"generation"`
	// Done counts the outputs already written, in file order.
	Done int `json:"done"`
	// Failures counts the settle attempts that failed in a row. The
	// runtime backs off by it.
	Failures int `json:"failures,omitempty"`
}

// State is a run's stored execution state: the engine's instance, the
// pending-outputs mark, the timeline, the step claim and the parallel
// branches.
type State struct {
	// Instance is the run's own instance. While the run has branches it
	// is inert: its state is empty, so [flow.Read] reads it as stranded,
	// and Previous names the state that forked.
	Instance flow.Instance
	// Pending is nil when the run waits in its state.
	Pending *Pending
	// Log is the timeline, at most [MaxLog] entries.
	Log []Step
	// Claim is nil when nobody is completing the step.
	Claim *Claim
	// Branches are the live parallel branches, in the order of the fork's
	// targets; empty for a run that has not forked. At most [MaxBranches].
	Branches []Branch
}

// MaxBranches caps [State.Branches]: the targets of one fork.
const MaxBranches = 8

// Branch is one parallel branch of a forked run. It carries what the run
// carries for its own instance: an instance, a pending mark and a claim.
// Generations are unique across the run, so a generation names one step.
type Branch struct {
	// Name is the state the branch started in: a target of the fork.
	Name     string        `json:"name"`
	Instance flow.Instance `json:"instance"`
	Pending  *Pending      `json:"pending,omitempty"`
	Claim    *Claim        `json:"claim,omitempty"`
	// Arrived reports that the branch reached the join and waits there
	// for the others.
	Arrived bool `json:"arrived,omitempty"`
}

// NewRun is a run to create.
type NewRun struct {
	ID      RunID
	Flow    string
	Version string
	// Definition is the version's definition, as the flows.yaml loader
	// hashed it. The first run of (Flow, Version) stores it in the same
	// step; a later one must carry the same bytes, or the create is
	// [ErrDefinitionConflict].
	Definition []byte
	Subject    entity.Ref
	Singleton  bool
	// At is the create time: CreatedAt and UpdatedAt.
	At time.Time
	// Instance is the starting state; the zero value is unset (see
	// [flow.Instance]).
	Instance flow.Instance
	// Pending is the starting pending-outputs mark, or nil.
	Pending *Pending
	// Log is the starting timeline, or nil.
	Log []Step
}

// Query selects runs for [Store.List]. Every set field narrows the result.
type Query struct {
	// Flow selects the runs of one flow.
	Flow string
	// SubjectID selects the runs on one subject id, on every face.
	SubjectID string
	// SubjectIDs selects the runs on any of these subject ids, on every
	// face: at most [MaxPage] ids. Empty: no narrowing.
	SubjectIDs []string
	// IDs selects the runs with any of these ids: at most [MaxPage] ids.
	// Empty: no narrowing.
	IDs []RunID
	// After resumes a listing: only runs with a greater id.
	After RunID
	// Limit is 1 to [MaxPage].
	Limit int
}

// DueCursor resumes [Store.ListDue] after the last run a call returned.
// The zero value starts at the beginning.
type DueCursor struct {
	At time.Time
	ID RunID
}

// Store persists the execution state of flow runs. See the package doc.
//
// Times are stored at microsecond precision in UTC ([flow.NormTime]), and
// read back that way.
//
// Nil: no method returns a nil error with a zero result for a missing run
// or definition; that is [ErrNotFound]. List methods return an empty,
// non-nil slice when nothing matches.
type Store interface {
	// Create stores a new run with Rev 1 and returns it. An id stored
	// already is [ErrExists]. A singleton run while another singleton
	// run of the flow is stored on the subject id, on any face, is
	// [ErrSubjectBusy]; a stored singleton run blocks until it is
	// deleted, so the runtime deletes a run when it ends. A non-singleton
	// run never blocks, so a flow switched from many to one keeps its
	// older runs. A subject id holding [MaxRunsPerSubject] runs of the
	// flow is [ErrSubjectFull]. The checks, the definition snapshot and
	// the insert are one step.
	Create(ctx context.Context, nr NewRun) (Run, error)

	// Get returns one run.
	Get(ctx context.Context, id RunID) (Run, error)

	// Update replaces the run's state (instance and pending mark) and sets
	// UpdatedAt to at, as one compare-and-set on expectRev, and returns
	// the run with its new rev. A run stored at another rev is
	// [ErrConflict]; a missing run is [ErrNotFound].
	Update(ctx context.Context, id RunID, expectRev int64, st State, at time.Time) (Run, error)

	// Delete drops the run's state, as one compare-and-set on expectRev:
	// the retention point when a run completes or is cancelled. A run
	// stored at another rev is [ErrConflict]; a missing run is
	// [ErrNotFound].
	Delete(ctx context.Context, id RunID, expectRev int64) error

	// ListDue returns the runs whose Instance.NextDueAt is set and at or
	// before until, ordered by due time and then id, after the cursor, at
	// most limit (1 to [MaxPage]). A caller that cannot advance a due run
	// passes the last run as the next cursor, so it does not starve the
	// runs behind it.
	ListDue(ctx context.Context, until time.Time, after DueCursor, limit int) ([]Run, error)

	// List returns the runs q selects, ordered by id.
	List(ctx context.Context, q Query) ([]Run, error)

	// RekeySubject moves every run on subject id oldID, on every face, to
	// newID, keeping each run's face and rev, and returns how many moved.
	// When a singleton run would meet another singleton run of its flow
	// on newID, or newID would exceed [MaxRunsPerSubject], nothing moves
	// and the result is [ErrSubjectBusy] or [ErrSubjectFull]. Moving
	// nothing is not an error, so a repeat after a crash is safe.
	RekeySubject(ctx context.Context, oldID, newID string) (int, error)

	// Definition returns the definition stored for a flow version.
	Definition(ctx context.Context, flow, version string) ([]byte, error)

	// PruneDefinitions drops the definitions stored before the given time
	// that no stored run uses, and returns how many it dropped. A create
	// running at the same time either keeps its definition or stores it
	// again.
	PruneDefinitions(ctx context.Context, before time.Time) (int, error)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// CheckID checks a run id: an entity id of at most [MaxIDLen] bytes.
func CheckID(id RunID) error {
	if len(id) > MaxIDLen {
		return invalid("run id longer than %d bytes", MaxIDLen)
	}
	if err := entity.ValidateID(string(id)); err != nil {
		return invalid("run id: %v", err)
	}
	return nil
}

// CheckSubjectID checks a subject id: an entity id of at most [MaxIDLen]
// bytes.
func CheckSubjectID(id string) error {
	if len(id) > MaxIDLen {
		return invalid("subject id longer than %d bytes", MaxIDLen)
	}
	if err := entity.ValidateID(id); err != nil {
		return invalid("subject id: %v", err)
	}
	return nil
}

// CheckFlow checks a flow name ([flow.ValidName]).
func CheckFlow(name string) error {
	if !flow.ValidName(name) {
		return invalid("flow name %q", name)
	}
	return nil
}

// CheckVersion checks a flow version: [VersionLen] lowercase hex digits.
func CheckVersion(v string) error {
	if len(v) != VersionLen || strings.Trim(v, "0123456789abcdef") != "" {
		return invalid("flow version %q is not %d lowercase hex digits", v, VersionLen)
	}
	return nil
}

func checkSubject(ref entity.Ref) error {
	if err := CheckSubjectID(ref.ID); err != nil {
		return err
	}
	if ref.Face.IsImplicit() {
		return nil
	}
	if _, err := entity.ParseFace(string(ref.Face)); err != nil {
		return invalid("subject face: %v", err)
	}
	return nil
}

// afterEpoch reports whether t is set and after the Unix epoch. The SQL
// backends store times as microseconds since the epoch with 0 for none,
// so a time at or before it would read back as another value.
func afterEpoch(t time.Time) bool { return flow.NormTime(t).UnixMicro() > 0 }

// CheckInstance checks a stored instance: valid state names, no negative
// counters, a due time after the epoch when set, and no more restart
// anchors than [flow.MaxTimers] or reminder records than
// [flow.MaxReminders].
func CheckInstance(inst flow.Instance) error {
	for _, s := range []string{inst.State, inst.Previous} {
		if s != "" && !flow.ValidName(s) {
			return invalid("state name %q", s)
		}
	}
	if inst.Generation < 0 || inst.Cause < 0 {
		return invalid("negative generation or cause")
	}
	if !inst.NextDueAt.IsZero() && !afterEpoch(inst.NextDueAt) {
		return invalid("due time at or before the epoch")
	}
	if len(inst.Restarts) > flow.MaxTimers {
		return invalid("more than %d restart anchors", flow.MaxTimers)
	}
	for name := range inst.Restarts {
		if !flow.ValidName(name) {
			return invalid("timer name %q", name)
		}
	}
	if len(inst.Reminders) > flow.MaxReminders {
		return invalid("more than %d reminder records", flow.MaxReminders)
	}
	for name, r := range inst.Reminders {
		if !flow.ValidName(name) {
			return invalid("reminder name %q", name)
		}
		if r.Count < 0 {
			return invalid("negative reminder count")
		}
	}
	return nil
}

// CheckState checks a stored state: its instance ([CheckInstance]) and a
// pending mark with a valid state name, a generation not below zero and
// Done from 0 to [MaxPendingDone].
func CheckState(st State) error {
	if err := CheckInstance(st.Instance); err != nil {
		return err
	}
	if err := checkLog(st.Log); err != nil {
		return err
	}
	if err := checkClaim(st.Claim); err != nil {
		return err
	}
	if err := checkPending(st.Pending); err != nil {
		return err
	}
	return checkBranches(st.Branches)
}

func checkClaim(c *Claim) error {
	if c != nil && (c.Generation < 0 || c.By == "" || len(c.By) > MaxClaimBy) {
		return invalid("claim out of range")
	}
	return nil
}

func checkPending(p *Pending) error {
	if p == nil {
		return nil
	}
	if !flow.ValidName(p.State) {
		return invalid("pending state name %q", p.State)
	}
	if p.Generation < 0 || p.Done < 0 || p.Done > MaxPendingDone || p.Failures < 0 ||
		p.Failures > MaxPendingFailures {

		return invalid("pending mark out of range")
	}
	return nil
}

// checkBranches checks the parallel branches: at most [MaxBranches], with
// distinct valid names, each checked like the run's own instance, mark
// and claim.
func checkBranches(bs []Branch) error {
	if len(bs) > MaxBranches {
		return invalid("more than %d branches", MaxBranches)
	}
	seen := map[string]bool{}
	for _, b := range bs {
		if !flow.ValidName(b.Name) || seen[b.Name] {
			return invalid("branch name %q", b.Name)
		}
		seen[b.Name] = true
		if err := CheckInstance(b.Instance); err != nil {
			return err
		}
		if err := checkClaim(b.Claim); err != nil {
			return err
		}
		if err := checkPending(b.Pending); err != nil {
			return err
		}
	}
	return nil
}

// checkLog checks a timeline: at most [MaxLog] entries, each with a valid
// state name and a known cause.
func checkLog(log []Step) error {
	if len(log) > MaxLog {
		return invalid("more than %d log entries", MaxLog)
	}
	for _, s := range log {
		if !flow.ValidName(s.State) || s.From != "" && !flow.ValidName(s.From) {
			return invalid("log state name %q", s.State)
		}
		if s.Timer != "" && !flow.ValidName(s.Timer) || s.Branch != "" && !flow.ValidName(s.Branch) ||
			len(s.By) > MaxClaimBy {

			return invalid("log entry out of range")
		}
		switch s.Via {
		case ViaStart, ViaDone, ViaChange, ViaTimer, ViaJoin:
		default:
			return invalid("log cause %q", s.Via)
		}
	}
	return nil
}

// AppendLog returns log with s added, trimmed to [MaxLog] entries: the
// first entry stays, and the oldest after it go.
func AppendLog(log []Step, s Step) []Step {
	out := append(slices.Clone(log), s)
	if len(out) > MaxLog {
		out = append(out[:1], out[len(out)-MaxLog+1:]...)
	}
	return out
}

// CheckNewRun checks every field of nr.
func CheckNewRun(nr NewRun) error {
	if err := CheckID(nr.ID); err != nil {
		return err
	}
	if err := CheckFlow(nr.Flow); err != nil {
		return err
	}
	if err := CheckVersion(nr.Version); err != nil {
		return err
	}
	if len(nr.Definition) == 0 || len(nr.Definition) > MaxDefinitionBytes {
		return invalid("definition must hold 1 to %d bytes", MaxDefinitionBytes)
	}
	if err := checkSubject(nr.Subject); err != nil {
		return err
	}
	if !afterEpoch(nr.At) {
		return invalid("create time is not after the epoch")
	}
	return CheckState(State{Instance: nr.Instance, Pending: nr.Pending, Log: nr.Log})
}

// CheckUpdate checks the arguments of [Store.Update].
func CheckUpdate(id RunID, expectRev int64, st State, at time.Time) error {
	if err := CheckID(id); err != nil {
		return err
	}
	if expectRev < 1 {
		return invalid("rev %d", expectRev)
	}
	if !afterEpoch(at) {
		return invalid("update time is not after the epoch")
	}
	return CheckState(st)
}

// CheckDelete checks the arguments of [Store.Delete].
func CheckDelete(id RunID, expectRev int64) error {
	if err := CheckID(id); err != nil {
		return err
	}
	if expectRev < 1 {
		return invalid("rev %d", expectRev)
	}
	return nil
}

func checkLimit(limit int) error {
	if limit < 1 || limit > MaxPage {
		return invalid("limit %d is not 1 to %d", limit, MaxPage)
	}
	return nil
}

// CheckDue checks the arguments of [Store.ListDue].
func CheckDue(until time.Time, after DueCursor, limit int) error {
	if until.IsZero() {
		return invalid("until is zero")
	}
	if after.ID != "" {
		if err := CheckID(after.ID); err != nil {
			return err
		}
	}
	return checkLimit(limit)
}

// CheckQuery checks q: a set Flow, SubjectID, SubjectIDs, IDs or After
// must be valid, SubjectIDs and IDs hold at most [MaxPage] ids each, and
// Limit must be 1 to [MaxPage].
func CheckQuery(q Query) error {
	if q.Flow != "" {
		if err := CheckFlow(q.Flow); err != nil {
			return err
		}
	}
	if q.SubjectID != "" {
		if err := CheckSubjectID(q.SubjectID); err != nil {
			return err
		}
	}
	if len(q.SubjectIDs) > MaxPage {
		return invalid("%d subject ids; at most %d", len(q.SubjectIDs), MaxPage)
	}
	for _, id := range q.SubjectIDs {
		if err := CheckSubjectID(id); err != nil {
			return err
		}
	}
	if len(q.IDs) > MaxPage {
		return invalid("%d run ids; at most %d", len(q.IDs), MaxPage)
	}
	for _, id := range q.IDs {
		if err := CheckID(id); err != nil {
			return err
		}
	}
	if q.After != "" {
		if err := CheckID(q.After); err != nil {
			return err
		}
	}
	return checkLimit(q.Limit)
}

// CheckRekey checks the arguments of [Store.RekeySubject].
func CheckRekey(oldID, newID string) error {
	if err := CheckSubjectID(oldID); err != nil {
		return err
	}
	return CheckSubjectID(newID)
}

// CheckDefinitionKey checks the arguments of [Store.Definition].
func CheckDefinitionKey(flowName, version string) error {
	if err := CheckFlow(flowName); err != nil {
		return err
	}
	return CheckVersion(version)
}

// Digest is the hex SHA-256 of a definition, which the backends compare
// instead of the bytes.
func Digest(def []byte) string {
	sum := sha256.Sum256(def)
	return hex.EncodeToString(sum[:])
}

// NormInstance returns inst with every time normalized ([flow.NormTime])
// and empty maps as nil, as every backend reads it back.
func NormInstance(inst flow.Instance) flow.Instance {
	out := inst
	out.EnteredAt = flow.NormTime(inst.EnteredAt)
	out.NextDueAt = flow.NormTime(inst.NextDueAt)
	out.Restarts = nil
	if len(inst.Restarts) > 0 {
		out.Restarts = make(map[string]time.Time, len(inst.Restarts))
		for k, v := range inst.Restarts {
			out.Restarts[k] = flow.NormTime(v)
		}
	}
	out.Reminders = nil
	if len(inst.Reminders) > 0 {
		out.Reminders = make(map[string]flow.ReminderRecord, len(inst.Reminders))
		for k, v := range inst.Reminders {
			out.Reminders[k] = flow.NormReminder(v)
		}
	}
	return out
}

// envelope is the stored form of a run's execution state. It is an
// object rather than the bare instance so later fields need no change to
// the tables; the pending mark is the first (RR-U30RNUZ).
type envelope struct {
	Instance flow.Instance `json:"instance"`
	Pending  *Pending      `json:"pending,omitempty"`
	Log      []Step        `json:"log,omitempty"`
	Claim    *Claim        `json:"claim,omitempty"`
	Branches []Branch      `json:"branches,omitempty"`
}

// NormState returns a deep copy of st with every instance and time
// normalized ([NormInstance], [flow.NormTime]).
func NormState(st State) State {
	out := State{Instance: NormInstance(st.Instance), Pending: copyPending(st.Pending), Claim: normClaim(st.Claim)}
	if len(st.Log) > 0 {
		out.Log = make([]Step, len(st.Log))
		for i, s := range st.Log {
			s.At = flow.NormTime(s.At)
			out.Log[i] = s
		}
	}
	if len(st.Branches) > 0 {
		out.Branches = make([]Branch, len(st.Branches))
		for i, b := range st.Branches {
			out.Branches[i] = Branch{Name: b.Name, Instance: NormInstance(b.Instance), Pending: copyPending(b.Pending),
				Claim: normClaim(b.Claim), Arrived: b.Arrived}
		}
	}
	return out
}

func copyPending(p *Pending) *Pending {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func normClaim(c *Claim) *Claim {
	if c == nil {
		return nil
	}
	cp := *c
	cp.At = flow.NormTime(c.At)
	return &cp
}

// EncodeState returns the stored form of st, normalized.
func EncodeState(st State) ([]byte, error) {
	n := NormState(st)
	return json.Marshal(envelope(n))
}

// DecodeState reads what [EncodeState] wrote.
func DecodeState(data []byte) (State, error) {
	var e envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return State{}, fmt.Errorf("flowrun: decode state: %w", err)
	}
	return NormState(State(e)), nil
}

// FromNew returns the run nr creates, with Rev 1 and every time
// normalized.
func FromNew(nr NewRun) Run {
	at := flow.NormTime(nr.At)
	st := NormState(State{Instance: nr.Instance, Pending: nr.Pending, Log: nr.Log})
	return Run{
		ID: nr.ID, Flow: nr.Flow, Version: nr.Version, Subject: nr.Subject, Singleton: nr.Singleton,
		CreatedAt: at, UpdatedAt: at, Rev: 1, State: st,
	}
}

// Before reports whether a run due at at with id comes after the
// cursor in [Store.ListDue] order.
func (c DueCursor) Before(at time.Time, id RunID) bool {
	if c.At.IsZero() && c.ID == "" {
		return true
	}
	ca := flow.NormTime(c.At)
	return at.After(ca) || (at.Equal(ca) && id > c.ID)
}

// SortDue sorts runs in [Store.ListDue] order.
func SortDue(runs []Run) {
	slices.SortFunc(runs, func(a, b Run) int {
		if c := a.Instance.NextDueAt.Compare(b.Instance.NextDueAt); c != 0 {
			return c
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
}

// CheckPrune checks the argument of [Store.PruneDefinitions].
func CheckPrune(before time.Time) error {
	if before.IsZero() {
		return invalid("before is zero")
	}
	return nil
}
