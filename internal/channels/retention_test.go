package channels

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTombstonePolicyValid(t *testing.T) {
	for _, p := range []TombstonePolicy{PolicyUnrecorded, PolicyDelete, PolicyDetach, PolicyOrphan} {
		assert.True(t, p.Valid(), "policy %q", p)
	}
	assert.False(t, TombstonePolicy("forever").Valid())
}

func TestFilingDueAtAndRecorded(t *testing.T) {
	assert.False(t, Filing{}.Recorded())
	assert.True(t, Filing{}.DueAt().IsZero())

	after := t0.Add(48 * time.Hour)
	early := t0.Add(time.Hour)
	f := Filing{PurgeAfter: after, TypeAfter: map[string]time.Time{"room": early, "topic": after.Add(time.Hour)}}
	assert.True(t, f.Recorded())
	assert.Equal(t, early, f.DueAt())
	assert.Equal(t, after, Filing{PurgeAfter: after}.DueAt())
}

func TestFilingRestorableAndConfirmed(t *testing.T) {
	cases := []struct {
		name       string
		f          Filing
		restorable bool
		confirmed  Filing
	}{
		{"unrecorded", Filing{}, true, Filing{}},
		{"pending delete", Filing{Policy: PolicyDelete, Pending: true}, true, Filing{Policy: PolicyDelete}},
		{"confirmed delete", Filing{Policy: PolicyDelete}, false, Filing{Policy: PolicyDelete}},
		{"confirmed detach", Filing{Policy: PolicyDetach}, false, Filing{Policy: PolicyDetach}},
		{"orphan", Filing{Policy: PolicyOrphan}, true, Filing{Policy: PolicyDetach}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.restorable, tc.f.Restorable())
			c := tc.f.Confirmed()
			assert.Equal(t, tc.confirmed, c)
			if tc.f.Policy != PolicyUnrecorded {
				assert.False(t, c.Restorable(), "a confirmed batch is final")
			}
		})
	}
}

func TestPrepareFiling(t *testing.T) {
	after := t0.Add(48 * time.Hour)
	early := t0.Add(time.Hour)
	cases := []struct {
		name    string
		in      Filing
		want    Filing
		wantErr error
	}{
		{"zero", Filing{}, Filing{}, nil},
		{"bad policy", Filing{Policy: "forever"}, Filing{}, ErrInvalidMessage},
		{"empty type map dropped", Filing{Policy: PolicyDelete, PurgeAfter: after, TypeAfter: map[string]time.Time{}},
			Filing{Policy: PolicyDelete, PurgeAfter: NormTime(after)}, nil},
		{"types without batch time", Filing{TypeAfter: map[string]time.Time{"room": early}}, Filing{}, ErrInvalidMessage},
		{"bad type", Filing{PurgeAfter: after, TypeAfter: map[string]time.Time{"a/b": early}}, Filing{}, ErrInvalidKey},
		{"early type kept, late and zero dropped",
			Filing{PurgeAfter: after, TypeAfter: map[string]time.Time{"room": early, "topic": after, "chat": {}}},
			Filing{PurgeAfter: NormTime(after), TypeAfter: map[string]time.Time{"room": NormTime(early)}}, nil},
		{"only late types", Filing{PurgeAfter: after, TypeAfter: map[string]time.Time{"topic": after.Add(time.Hour)}},
			Filing{PurgeAfter: NormTime(after)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PrepareFiling(tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFileBatch(t *testing.T) {
	f, err := FileBatch(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, Filing{}, f)

	want := Filing{Policy: PolicyDetach, PurgeAfter: NormTime(t0)}
	f, err = FileBatch(FileAs(Filing{Policy: PolicyDetach, PurgeAfter: t0}), nil)
	require.NoError(t, err)
	assert.Equal(t, want, f)

	boom := errors.New("boom")
	_, err = FileBatch(func([]Channel) (Filing, error) { return Filing{}, boom }, nil)
	require.ErrorIs(t, err, boom)

	_, err = FileBatch(FileAs(Filing{Policy: "forever"}), nil)
	require.ErrorIs(t, err, ErrInvalidMessage)
}

func TestCheckPurgeAndDropType(t *testing.T) {
	require.NoError(t, CheckPurge([]string{"room"}, alice, t0))
	require.NoError(t, CheckPurge(nil, alice, t0))
	require.ErrorIs(t, CheckPurge([]string{"a/b"}, alice, t0), ErrInvalidKey)
	require.ErrorIs(t, CheckPurge(nil, Author{}, t0), ErrInvalidMessage)
	require.ErrorIs(t, CheckPurge(nil, alice, time.Time{}), ErrInvalidMessage)

	require.NoError(t, CheckDropType("room"))
	require.ErrorIs(t, CheckDropType("a/b"), ErrInvalidKey)
}

func TestPurgeAudit(t *testing.T) {
	stamp := TombstoneStamp(t0)
	h := NewTombstoneHandle(Anchor{ID: "TKT-1", Face: "draft"}, stamp)
	assert.Equal(t, Anchor{ID: "TKT-1", Face: "draft"}, h.Anchor())
	assert.Equal(t, stamp, h.Stamp())

	recs := PurgeAudit(h, nil, alice, t0)
	require.Len(t, recs, 1)
	assert.Equal(t, AuditRecord{At: NormTime(t0), Op: AuditPurge, Face: "draft", By: alice, Tomb: stamp}, recs[0])

	chs := []Channel{
		{Key: Key{Anchor: Anchor{ID: "TKT-1", Face: "draft"}, Type: "room"}, LastSeq: 3},
		{Key: Key{Anchor: Anchor{ID: "TKT-1", Face: "draft"}, Type: "topic", Name: "abc123"}, LastSeq: 7},
	}
	recs = PurgeAudit(h, chs, alice, t0)
	require.Len(t, recs, 2)
	for i, r := range recs {
		assert.Equal(t, AuditPurge, r.Op)
		assert.Equal(t, chs[i].Key.Type, r.Type)
		assert.Equal(t, chs[i].Key.Name, r.Name)
		assert.Equal(t, chs[i].LastSeq, r.Seq)
		assert.Equal(t, alice, r.By)
		assert.Equal(t, stamp, r.Tomb)
		assert.Equal(t, NormTime(t0), r.At)
	}
}

func TestPurgeTypes(t *testing.T) {
	chs := []Channel{
		{Key: Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}},
		{Key: Key{Anchor: Anchor{ID: "TKT-1"}, Type: "topic", Name: "abc123"}},
	}
	assert.Equal(t, chs, PurgeTypes(chs, nil))
	assert.Equal(t, chs[1:], PurgeTypes(chs, []string{"topic"}))
	assert.Empty(t, PurgeTypes(chs, []string{"chat"}))
}

func TestStampTimeAndBefore(t *testing.T) {
	stamp := TombstoneStamp(t0)
	got, err := StampTime(stamp)
	require.NoError(t, err)
	assert.Equal(t, NormTime(t0), got)

	_, err = StampTime("nope")
	require.ErrorIs(t, err, ErrInvalidKey)
	// Matches the pattern but is no real date.
	_, err = StampTime("20261399T999999.000000Z-abcdef")
	require.ErrorIs(t, err, ErrInvalidKey)

	assert.Less(t, stamp, StampBefore(t0.Add(time.Microsecond)))
	assert.GreaterOrEqual(t, stamp, StampBefore(t0))
	assert.Equal(t, "purged-"+stamp, PurgedTrail(stamp))
}

func TestNewChannelCap(t *testing.T) {
	assert.Equal(t, MaxChannelsPerType, NewChannel{}.Cap())
	assert.Equal(t, MaxChannelsPerType, NewChannel{Max: MaxChannelsPerType + 1}.Cap())
	assert.Equal(t, 3, NewChannel{Max: 3}.Cap())
}

func TestCheckReleaseAndActionAudit(t *testing.T) {
	require.NoError(t, CheckRelease(ActionNone, Action{Status: ActionPending}))
	require.NoError(t, CheckRelease(ActionPending, Action{Status: ActionDone}))
	require.NoError(t, CheckRelease(ActionPending, Action{By: alice, At: t0}))
	require.ErrorIs(t, CheckRelease(ActionPending, Action{At: t0}), ErrInvalidMessage)
	require.ErrorIs(t, CheckRelease(ActionPending, Action{By: alice}), ErrInvalidMessage)

	k := Key{Anchor: Anchor{ID: "TKT-1"}, Type: "room"}
	_, ok := ActionAudit(k, 4, "promote", ActionNone, Action{Status: ActionPending, By: alice, At: t0})
	assert.False(t, ok, "taking an action pending is not recorded")

	rec, ok := ActionAudit(k, 4, "promote", ActionPending, Action{Status: ActionDone, By: alice, At: t0})
	require.True(t, ok)
	assert.Equal(t, AuditAction, rec.Op)
	assert.Equal(t, int64(4), rec.Seq)
	assert.Equal(t, "promote", rec.Action)
	assert.Empty(t, rec.Status)

	rec, ok = ActionAudit(k, 4, "promote", ActionPending, Action{By: alice, At: t0})
	require.True(t, ok)
	assert.Equal(t, AuditReleased, rec.Status)
}
