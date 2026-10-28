package channels

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCheckMentions(t *testing.T) {
	login := func(s string) Mention { return Mention{Target: s, Form: FormLogin} }
	got, err := CheckMentions([]Mention{login("b"), {Target: "USR-7", Form: FormUser, Span: "USR-7"}, login("b"),
		{Target: "a", Form: FormJob, Span: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Mention{{Target: "USR-7", Form: FormUser}, {Target: "a", Form: FormJob, Span: "x"}, login("b")}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("CheckMentions = %v; want %v (deduplicated, sorted, Span cleared when equal)", got, want)
	}
	if got, err := CheckMentions(nil); got != nil || err != nil {
		t.Errorf("CheckMentions(nil) = %v, %v", got, err)
	}
	for _, m := range []Mention{
		login(""), login(" a"), login("a\n"), login("a\x00"), login("\xff"), {Target: "a", Form: "role"},
		{Target: "a", Form: FormLogin, Span: " a"},
	} {
		if _, err := CheckMentions([]Mention{m}); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("CheckMentions(%q) = %v; want ErrInvalidMessage", m, err)
		}
	}
	var many []Mention
	for i := range MaxMentions + 1 {
		many = append(many, login(fmt.Sprint("u", i)))
	}
	if _, err := CheckMentions(many); !errors.Is(err, ErrTooLarge) {
		t.Errorf("11 targets: %v; want ErrTooLarge", err)
	}
	if _, err := CheckMentions(append(many[:MaxMentions:MaxMentions], many[0])); err != nil {
		t.Errorf("10 distinct targets with a duplicate: %v", err)
	}
}

func TestReplyTarget(t *testing.T) {
	alice := Author{User: "alice", OrgID: "a"}
	for _, tc := range []struct {
		name   string
		parent Message
		poster Author
		want   string
	}{
		{"another user", Message{Author: Author{User: "bob"}}, alice, "bob"},
		{"own message", Message{Author: Author{User: "alice", Tool: "mcp"}}, alice, ""},
		{"same user, other org", Message{Author: Author{User: "alice", OrgID: "b"}}, alice, ""},
		{"trigger post", Message{Author: Author{User: "bob"}, TriggeredBy: "trigger:x"}, alice, ""},
		{"reminder", Message{Author: Author{User: "bob"}, TriggeredBy: ReminderMarker + "x"}, alice, ""},
		{"empty author", Message{}, alice, ""},
		{"unknown author", Message{Author: Author{User: "unknown"}}, alice, ""},
		{"rela identity", Message{Author: Author{User: "system:realtime"}}, alice, ""},
		{"job identity", Message{Author: Author{User: "integration:bot"}}, alice, "integration:bot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ReplyTarget(tc.parent, tc.poster)
			if got != tc.want || ok != (tc.want != "") {
				t.Errorf("ReplyTarget = %q, %v; want %q", got, ok, tc.want)
			}
		})
	}
}

func TestEvict(t *testing.T) {
	rows := []AddressedRow{
		{Seq: 1, Target: "t", Author: "a"}, {Seq: 2, Target: "t", Author: "b"}, {Seq: 3, Target: "t", Author: "a"},
	}
	markers := func(ms ...int64) func(string) []int64 { return func(string) []int64 { return ms } }
	for _, tc := range []struct {
		name    string
		markers func(string) []int64
		author  string
		want    int
	}{
		{"no marker, other poster", markers(), "c", -1},
		{"no marker, same poster", markers(), "b", 1},
		{"read under every marker", markers(2, 5), "c", 0},
		{"unread under one marker", markers(0, 5), "a", 0},
		{"unread under one marker, other poster", markers(0, 5), "c", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Evict(rows, tc.markers, tc.author)
			if got != tc.want || ok != (tc.want >= 0) {
				t.Errorf("Evict = %d, %v; want %d", got, ok, tc.want)
			}
		})
	}
}

func TestSortAddressed(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	ka := Key{Anchor: Anchor{ID: "A"}, Type: "room"}
	kb := Key{Anchor: Anchor{ID: "B"}, Type: "room"}
	as := []Addressed{
		{Channel: Channel{Key: kb}, Seq: 1, At: t0, Kind: AddressMention},
		{Channel: Channel{Key: ka}, Seq: 2, At: t0, Kind: AddressReply},
		{Channel: Channel{Key: ka}, Seq: 2, At: t0, Kind: AddressMention},
		{Channel: Channel{Key: ka}, Seq: 1, At: t0, Kind: AddressMention},
		{Channel: Channel{Key: kb}, Seq: 9, At: t0.Add(time.Second), Kind: AddressMention},
	}
	SortAddressed(as)
	var got []string
	for _, a := range as {
		got = append(got, fmt.Sprintf("%s/%d/%s", a.Channel.Key.ID, a.Seq, a.Kind))
	}
	want := "[B/9/mention A/1/mention A/2/mention A/2/reply B/1/mention]"
	if fmt.Sprint(got) != want {
		t.Errorf("SortAddressed = %v; want %v", got, want)
	}
}

func TestCheckListAddressed(t *testing.T) {
	r := Reader{User: "u"}
	for _, kind := range []AddressKind{AddressAny, AddressMention, AddressReply} {
		for _, limit := range []int{1, MaxAddressedScan} {
			if err := CheckListAddressed(r, kind, limit); err != nil {
				t.Errorf("kind %q limit %d: %v", kind, limit, err)
			}
		}
	}
	for _, limit := range []int{0, MaxAddressedScan + 1} {
		if err := CheckListAddressed(r, AddressAny, limit); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("limit %d: %v; want ErrInvalidMessage", limit, err)
		}
	}
	if err := CheckListAddressed(r, "group", 1); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("unknown kind: %v; want ErrInvalidMessage", err)
	}
	if err := CheckListAddressed(Reader{}, AddressAny, 1); err == nil {
		t.Error("an empty reader was accepted")
	}
}

func TestAddressKindAdmits(t *testing.T) {
	for _, tc := range []struct {
		kind, row AddressKind
		want      bool
	}{
		{AddressAny, AddressMention, true}, {AddressAny, AddressReply, true},
		{AddressMention, AddressMention, true}, {AddressMention, AddressReply, false},
		{AddressReply, AddressReply, true}, {AddressReply, AddressMention, false},
	} {
		if got := tc.kind.Admits(tc.row); got != tc.want {
			t.Errorf("%q.Admits(%q) = %v; want %v", tc.kind, tc.row, got, tc.want)
		}
	}
}

func TestAppendRowsAndAddedMentions(t *testing.T) {
	m := Message{Seq: 3, Author: Author{User: "a"}, Mentions: []Mention{{Target: "x"}, {Target: "y"}}}
	rows := AppendRows(m, "p")
	if len(rows) != 3 || rows[2] != (AddressedRow{Seq: 3, Target: "p", Kind: AddressReply, Author: "a"}) {
		t.Errorf("AppendRows = %v", rows)
	}
	if AppendRows(Message{}, "") != nil {
		t.Error("AppendRows of no targets is not nil")
	}
	added := AddedMentions([]Mention{{Target: "x"}}, m.Mentions)
	if len(added) != 1 || added[0].Target != "y" {
		t.Errorf("AddedMentions = %v", added)
	}
	if AddedMentions(m.Mentions, m.Mentions) != nil {
		t.Error("AddedMentions of an unchanged set is not nil")
	}
}
