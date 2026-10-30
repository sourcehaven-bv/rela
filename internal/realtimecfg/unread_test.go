package realtimecfg

import (
	"testing"
)

// unreadRoom is a realtime.yaml with one log type, progress, and one signal
// type, typing. line is appended to progress's mapping and sig to typing's.
func unreadRoom(line, sig string) string {
	return "version: 1\nchannel_types:\n  room:\n    attach_to: [datalek]\n    roles: {m: {from: attached}}\n" +
		"    message_types:\n      progress:\n        post: [m]\n" + line +
		"      typing:\n        persist: ephemeral\n        post: [m]\n" + sig
}

// TestParse_Unread pins `unread:` on a message type (TKT-O6RC8Z AC 1): a
// YAML boolean on a log type, NoUnread set only by false, refused on a
// signal type, refused for a quoted value and a YAML 1.1 word as for
// `confirm:`, and named hints for quiet and display.
func TestParse_Unread(t *testing.T) {
	const mtKey = "channel_types.room.message_types.progress"
	for _, tc := range []struct {
		name, line, sig string
		noUnread        bool
		key, reason     string
	}{
		{name: "absent"},
		{name: "true", line: "        unread: true\n"},
		{name: "false", line: "        unread: false\n", noUnread: true},
		{name: "tagged bool", line: "        unread: !!bool false\n", noUnread: true},
		{name: "null is absent", line: "        unread: ~\n"},
		{
			name: "unquoted no", line: "        unread: no\n", key: mtKey + ".unread",
			reason: `"no" is not a boolean; use true or false`,
		},
		{
			name: "unquoted Off", line: "        unread: Off\n", key: mtKey + ".unread",
			reason: `"Off" is not a boolean; use true or false`,
		},
		{name: "quoted no", line: "        unread: \"no\"\n", key: mtKey + ".unread", reason: "must be true or false"},
		{name: "quoted false", line: "        unread: \"false\"\n", key: mtKey + ".unread", reason: "must be true or false"},
		{name: "tagged str", line: "        unread: !!str no\n", key: mtKey + ".unread", reason: "must be true or false"},
		{name: "string", line: "        unread: maybe\n", key: mtKey + ".unread", reason: "must be true or false"},
		{name: "integer", line: "        unread: 1\n", key: mtKey + ".unread", reason: "must be true or false"},
		{name: "list", line: "        unread: [false]\n", key: mtKey + ".unread", reason: "single value"},
		{
			name: "on an ephemeral type", sig: "        unread: false\n",
			key:    "channel_types.room.message_types.typing.unread",
			reason: "a signal is never stored, so it never counts as unread",
		},
		{
			name: "true on an ephemeral type", sig: "        unread: true\n",
			key:    "channel_types.room.message_types.typing.unread",
			reason: "a signal is never stored, so it never counts as unread",
		},
		{
			name: "quiet", line: "        quiet: true\n", key: mtKey + ".quiet",
			reason: "use unread: false to keep the type out of unread counts; to fold or hide it in the web app, " +
				"use display in data-entry.yaml",
		},
		{
			name: "display", line: "        display: hidden\n", key: mtKey + ".display",
			reason: "display belongs in data-entry.yaml: channels.<type>.messages.<mt>.display",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(unreadRoom(tc.line, tc.sig)), testMeta(t), nil)
			if tc.key != "" {
				assertProblem(t, err, tc.key, tc.reason)
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("room")
			if got := ct.MessageTypes["progress"].NoUnread; got != tc.noUnread {
				t.Errorf("NoUnread = %v; want %v", got, tc.noUnread)
			}
			if ct.MessageTypes["typing"].NoUnread {
				t.Error("the signal type has NoUnread")
			}
		})
	}
}

// TestCheckReload_UnreadFlip pins that a reload which flips `unread:` either
// way is accepted: the key is not part of a stored channel's shape.
func TestCheckReload_UnreadFlip(t *testing.T) {
	meta := testMeta(t)
	counting, err := Parse([]byte(unreadRoom("", "")), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	quiet, err := Parse([]byte(unreadRoom("        unread: false\n", "")), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckReload(counting, quiet); err != nil {
		t.Errorf("flip to false refused: %v", err)
	}
	if err := CheckReload(quiet, counting); err != nil {
		t.Errorf("flip to true refused: %v", err)
	}
}
