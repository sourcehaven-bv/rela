package acl

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// jobGrantsPolicy grants system:bot a role with field and option blocks,
// gives everyone update on note, and maps a claim to a role that the bot
// must not receive.
const jobGrantsPolicy = `
roles:
  bot:
    read: [ticket, "page@draft", note]
    create: [ticket, "page@draft"]
    update: [ticket, "page@draft"]
    permissions: [links.write]
    fields:
      ticket:
        - field: status
        - field: owner
          when: "entity.status == 'open'"
    options:
      ticket:
        - field: severity
          option: low
  everyone:
    read: [note]
    update: [note]
  claimed:
    read: ["*"]
    update: ["*"]
assignments:
  system:bot: bot
asserted_role_assignments:
  system:bot: [claimed]
relation_grants:
  links:
    create: links.write
role_relations:
  owns:
    confers: claimed
    requires_permission: owners.write
`

// TestJobIdentityGrants pins what JobGrants answers: assignment and
// everyone count, asserted roles do not, field and option blocks fold by
// the union rule, a when: counts as granted, and a relation follows
// relation_grants, the delegate permission and the source create verb.
func TestJobIdentityGrants(t *testing.T) {
	p := mustPolicy(t, jobGrantsPolicy)
	g := JobIdentityGrants(p, "system:bot")
	none := JobIdentityGrants(p, "system:other")
	draft := entity.Face("draft")
	checks := []struct {
		name string
		got  bool
		want bool
	}{
		{"update ticket", g.Verb(OpUpdate, "ticket", ""), true},
		{"create ticket", g.Verb(OpCreate, "ticket", ""), true},
		{"delete is never answered", g.Verb(OpDelete, "ticket", ""), false},
		{"update page@draft", g.Verb(OpUpdate, "page", draft), true},
		{"update page@published", g.Verb(OpUpdate, "page", "published"), false},
		{"everyone grants note", g.Verb(OpUpdate, "note", ""), true},
		{"asserted role does not count", g.Verb(OpUpdate, "risk", ""), false},
		{"unassigned identity holds only everyone", none.Verb(OpUpdate, "ticket", ""), false},
		{"unassigned identity holds everyone's note", none.Verb(OpUpdate, "note", ""), true},
		{"granted field", g.FieldWrite(OpUpdate, "ticket", "", "status"), true},
		{"field with when counts", g.FieldWrite(OpUpdate, "ticket", "", "owner"), true},
		{"field outside the block", g.FieldWrite(OpUpdate, "ticket", "", "title"), false},
		{"no block opens every field", g.FieldWrite(OpUpdate, "page", draft, "title"), true},
		{"granted option", g.Option(OpUpdate, "ticket", "", "severity", "low"), true},
		{"option outside the block", g.Option(OpUpdate, "ticket", "", "severity", "high"), false},
		{"field without options is open", g.Option(OpUpdate, "ticket", "", "kind", "x"), true},
		{"relation grant", g.Relates("links", "ticket", "", nil), true},
		{"relation by source create", g.Relates("about", "ticket", "", nil), true},
		{"relation on a face the bot cannot create", g.Relates("about", "page", "", []entity.Face{draft, "published"}),
			false},
		{"relation at a granted tail", g.Relates("about", "page", draft, nil), true},
		{"delegate permission missing", g.Relates("owns", "ticket", "", nil), false},
		{"nil policy grants nothing", JobIdentityGrants(nil, "system:bot").Relates("links", "ticket", "", nil), false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v; want %v", c.name, c.got, c.want)
		}
	}
}

// TestJobIdentityGrants_UnionRule pins the union rule: a role without a
// fields block that holds update opens every field, even beside a role
// whose block omits it, and only the blocked role leaves it closed.
func TestJobIdentityGrants_UnionRule(t *testing.T) {
	const union = `
roles:
  bot:
    read: [ticket]
    update: [ticket]
    fields:
      ticket: [{field: status}]
  everyone:
    read: [ticket]
    update: [ticket]
assignments:
  system:bot: bot
`
	if !JobIdentityGrants(mustPolicy(t, union), "system:bot").FieldWrite(OpUpdate, "ticket", "", "title") {
		t.Error("everyone's update without a block opens title")
	}
	const blocked = `
roles:
  bot:
    read: [ticket]
    update: [ticket]
    fields:
      ticket: [{field: status}]
  everyone:
    read: [ticket]
assignments:
  system:bot: bot
`
	g := JobIdentityGrants(mustPolicy(t, blocked), "system:bot")
	if g.FieldWrite(OpUpdate, "ticket", "", "title") {
		t.Error("only the blocked role applies: title stays closed")
	}
	if !g.FieldWrite(OpCreate, "ticket", "", "status") {
		t.Error("the block grants status on create too")
	}
}

// TestAnyRoleGrants pins the policy-wide floor of the realtime answer
// grant warning: any role, everyone included, whatever it is assigned to,
// grants the verb, and a field block closes a field no role opens.
func TestAnyRoleGrants(t *testing.T) {
	p := mustPolicy(t, `
roles:
  writer:
    read: [ticket]
    update: [ticket]
    fields:
      ticket:
        - field: status
  reader:
    read: [ticket]
  everyone:
    read: [note]
    update: [note]
`)
	cases := []struct {
		name  string
		p     *Policy
		op    Op
		typ   string
		field string
		want  bool
	}{
		{"unassigned role grants update", p, OpUpdate, "ticket", "", true},
		{"field in the block", p, OpUpdate, "ticket", "status", true},
		{"field the block closes", p, OpUpdate, "ticket", "owner", false},
		{"no role grants create", p, OpCreate, "ticket", "", false},
		{"everyone grants update", p, OpUpdate, "note", "body", true},
		{"unknown type", p, OpUpdate, "page", "", false},
		{"delete is not answered", p, OpDelete, "ticket", "", false},
		{"nil policy", nil, OpUpdate, "ticket", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnyRoleGrants(tc.p, tc.op, tc.typ, "", tc.field); got != tc.want {
				t.Errorf("AnyRoleGrants = %v; want %v", got, tc.want)
			}
		})
	}
}
