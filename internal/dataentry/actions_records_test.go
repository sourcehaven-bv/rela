package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// principalAppender records who appended. The owner check behind the real
// appender is the record service's, tested in appbuild; this pins that an
// action script reaches the appender as the calling user.
type principalAppender struct{ users []string }

func (p *principalAppender) AppendRecords(ctx context.Context, _ string, rows []lua.RecordRow) (
	[]lua.AppendedRecord, error,
) {
	p.users = append(p.users, principal.From(ctx).User)
	out := make([]lua.AppendedRecord, len(rows))
	for i, r := range rows {
		out[i] = lua.AppendedRecord{ID: "R-1", Owner: r.Owner}
	}
	return out, nil
}

const appendRecordsCall = `local rows, err = rela.append_records("measurement", {{owner = "AUD-1", fields = {value = 1}}})
if not rows then return { message = "ERR " .. err } end
return { message = "OK " .. rows[1].owner }`

func TestActionScript_AppendsRecordsAsCaller(t *testing.T) {
	a := newAttachApp(t)
	fake := &principalAppender{}
	a.app.records = &recordsHandler{appender: fake}
	if msg := a.runAction(t, "alice", appendRecordsCall); msg != "OK AUD-1" {
		t.Fatalf("message %q, want OK AUD-1", msg)
	}
	if len(fake.users) != 1 || fake.users[0] != "alice" {
		t.Errorf("appended as %v, want [alice]", fake.users)
	}
}

func TestActionScript_NoAppendRecordsWithoutRecords(t *testing.T) {
	a := newAttachApp(t)
	if msg := a.runAction(t, "alice", `return { message = type(rela.append_records) }`); msg != "nil" {
		t.Errorf("rela.append_records is %q without record types, want nil", msg)
	}
}
