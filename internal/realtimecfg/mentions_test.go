package realtimecfg

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// mentionRoom is a realtime.yaml with one message type whose property
// props.to has the given type and notify line.
func mentionRoom(propType, notify string) string {
	to := ""
	if propType == "ref" {
		to = ", to: [risico]"
	}
	return "version: 1\nchannel_types:\n  room:\n    attach_to: [datalek]\n    roles: {m: {from: attached}}\n" +
		"    message_types:\n      handoff:\n        post: [m]\n        properties:\n" +
		"          to: {type: " + propType + to + notify + "}\n"
}

// TestCompile_NotifyOnlyOnRef pins `notify:` (TKT-6RXAC0 AC 19): it is
// accepted on a ref property and sets NoNotify when false; on any other
// type it is a config error naming the property path.
func TestCompile_NotifyOnlyOnRef(t *testing.T) {
	for _, tc := range []struct {
		notify   string
		noNotify bool
	}{{"", false}, {", notify: true", false}, {", notify: false", true}} {
		c, err := Parse([]byte(mentionRoom("ref", tc.notify)), testMeta(t), nil)
		if err != nil {
			t.Fatalf("%q: %v", tc.notify, err)
		}
		ct, _ := c.ChannelType("room")
		if got := ct.MessageTypes["handoff"].Properties["to"].NoNotify; got != tc.noNotify {
			t.Errorf("%q: NoNotify = %v; want %v", tc.notify, got, tc.noNotify)
		}
	}
	for _, typ := range []string{"string", "markdown", "integer"} {
		_, err := Parse([]byte(mentionRoom(typ, ", notify: false")), testMeta(t), nil)
		assertProblem(t, err, "channel_types.room.message_types.handoff.properties.to.notify", "only a ref property")
	}
}

// TestCompiled_MentionUserType pins the one predicate of the mention form
// (AC 19): the user entity type only when acl.yaml sets both
// user_entity_type and principal_property.
func TestCompiled_MentionUserType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy *acl.Policy
		want   string
	}{
		{"no acl.yaml", nil, ""},
		{"user_entity_type only", &acl.Policy{UserEntityType: "risico"}, ""},
		{"principal_property only", &acl.Policy{PrincipalProperty: "login"}, ""},
		{"both", &acl.Policy{UserEntityType: "risico", PrincipalProperty: "login"}, "risico"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte("version: 1\n"), testMeta(t), tc.policy)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.MentionUserType(); got != tc.want {
				t.Errorf("MentionUserType() = %q; want %q", got, tc.want)
			}
		})
	}
	var nilCompiled *Compiled
	if nilCompiled.MentionUserType() != "" {
		t.Error("a nil Compiled has a mention user type")
	}
}
