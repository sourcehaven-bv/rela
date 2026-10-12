package lease

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// User-facing lease texts. The CLI and the MCP tools both answer through
// [UserText], so their texts cannot drift apart. The HTTP API maps the same
// errors to status codes and codes of its own. No text carries a lease id.
const (
	// NotEnabledText answers [ErrNotEnabled].
	NotEnabledText = "leases are not enabled on this project"
	// LostText answers [ErrLost].
	LostText = "lease lost: it expired, was released, or you no longer have access to the entity. " +
		"Claim the entity again."
	// FullText answers [ErrFull].
	FullText = "the lease store is full; retry later"
	// FailedText answers an error [UserText] does not know. The caller logs
	// the error and shows this text instead.
	FailedText = "the lease operation failed; retry later"
	// HeldPrefix starts the answer to a [HeldError]; a JSON object follows.
	HeldPrefix = "lease held: "
)

// ErrNotEnabled reports a project assembled without a lease backend. The
// service never returns it; a surface with no service does.
var ErrNotEnabled = errors.New("lease: leases are not enabled")

// AmbiguousError reports a bare id of an entity with more than one face the
// caller may read, so a claim must name the face. Err is the gate's own
// error and gives the text; the wiring site wraps it, so package lease does
// not import the package that defines it.
type AmbiguousError struct {
	Err error
}

func (e *AmbiguousError) Error() string { return e.Err.Error() }

// Unwrap returns the gate's error.
func (e *AmbiguousError) Unwrap() error { return e.Err }

// heldJSON is the body of a `lease held:` answer. The status and label are
// written by the holder.
type heldJSON struct {
	Holder    holderJSON `json:"holder"`
	Since     string     `json:"since"`
	ExpiresAt string     `json:"expires_at"`
	Status    string     `json:"status,omitempty"`
}

// holderJSON names the holder in a `lease held:` answer.
type holderJSON struct {
	User  string `json:"user"`
	Tool  string `json:"tool"`
	Label string `json:"label,omitempty"`
}

// UserText returns the text a user sees for err, a lease error from the
// [Service] or [ErrNotEnabled]. listHint names the surface's own way to
// list leases (such as "`rela lease list`"); the limit text cites it.
//
// ok is false for an error UserText does not know: the caller logs it and
// shows [FailedText]. It is also false for store.ErrNotFound. A miss is not
// handled here because each surface answers it with the text its own entity
// read gives for the same id, so a claim tells a hidden entity from a
// missing one no better than that read does. Callers check for a miss first.
func UserText(err error, listHint string) (text string, ok bool) {
	if held, isHeld := errors.AsType[*HeldError](err); isHeld {
		return heldText(held.Current), true
	}
	if _, isAmb := errors.AsType[*AmbiguousError](err); isAmb {
		return err.Error(), true
	}
	switch {
	case errors.Is(err, ErrNotEnabled):
		return NotEnabledText, true
	case errors.Is(err, ErrLost):
		return LostText, true
	case errors.Is(err, ErrLimit):
		return limitText(listHint), true
	case errors.Is(err, ErrFull):
		return FullText, true
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrNoPrincipal), errors.Is(err, acl.ErrForbidden):
		return err.Error(), true
	}
	return "", false
}

func limitText(listHint string) string {
	return fmt.Sprintf("lease limit reached: you hold %d live leases, the most one user may hold. "+
		"The cap is shared by every agent running as you. Release leases you no longer need "+
		"(%s shows them) and claim again.", MaxPerIdentity, listHint)
}

func heldText(cur Lease) string {
	text, err := json.Marshal(heldJSON{
		Holder: holderJSON{User: cur.Holder.User, Tool: cur.Holder.Tool, Label: cur.Holder.Label},
		Since:  textTime(cur.Since), ExpiresAt: textTime(cur.Expires), Status: cur.Status,
	})
	if err != nil { // coverage-ignore: a struct of strings always marshals
		return FailedText
	}
	return HeldPrefix + string(text)
}

func textTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }
