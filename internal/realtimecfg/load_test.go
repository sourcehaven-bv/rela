package realtimecfg

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	data, err := os.ReadFile("testdata/schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := metamodel.Parse(data)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return m
}

func testPolicy(t *testing.T) *acl.Policy {
	t.Helper()
	data, err := os.ReadFile("testdata/acl.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := acl.ParsePolicy(data, "acl.yaml")
	if err != nil {
		t.Fatalf("parse acl: %v", err)
	}
	return p
}

func golden(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParse_Golden(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	l := c.Limits()
	if l.LogRate != (realtimelimits.Rate{Count: 20, Per: time.Minute}) || l.PageLimit != 100 {
		t.Errorf("limits = %+v; want post_rate 20/m and page_limit 100", l)
	}
	if l.EphemeralRate != realtimelimits.DefaultEphemeralRate || l.MessageBytes != realtimelimits.MaxMessageBytes {
		t.Errorf("unset limits did not keep the built-in values: %+v", l)
	}

	names := []string{}
	for _, ct := range c.ChannelTypes() {
		names = append(names, ct.Name)
	}
	if !slices.Equal(names, []string{"breach-response", "incident-room", "page-threads"}) {
		t.Fatalf("channel types = %v", names)
	}

	if br, _ := c.ChannelType("breach-response"); br.Session() == nil || !br.Session().HasTimers() ||
		len(br.Session().Reminders()) != 1 || br.Session().OnFinal() != OnFinalClose {

		t.Error("breach-response lost its session timers, reminders or on_final")
	}

	room, ok := c.ChannelType("incident-room")
	if !ok {
		t.Fatal("incident-room missing")
	}
	if room.Anchor != AnchorFamily || room.Cardinality != CardinalityOne || room.Create != nil {
		t.Errorf("room anchor/cardinality/create = %s/%s/%v", room.Anchor, room.Cardinality, room.Create)
	}
	if room.OnEntityDelete != OnDeleteBlock {
		t.Errorf("on_entity_delete = %s", room.OnEntityDelete)
	}
	if room.Read.Source.Kind != SourceAttached || !room.Read.RequireAnchorRead {
		t.Errorf("read = %+v; want an attached source that requires anchor read", room.Read)
	}
	for _, name := range []string{"lead", "dpo", "member", "agent"} {
		if room.Roles[name].RequireAnchorRead {
			t.Errorf("role %s requires anchor read; only Read and conditional gates do", name)
		}
	}
	wantKinds := map[string]SourceKind{
		"lead": SourceEntityProperty, "dpo": SourcePermission, "member": SourceAttached, "agent": SourcePrincipalType,
	}
	for name, want := range wantKinds {
		if got := room.Roles[name].Source.Kind; got != want {
			t.Errorf("role %s source kind = %d; want %d", name, got, want)
		}
	}
	if room.Roles["lead"].Source.Property != "owner" || room.Roles["dpo"].Source.Permission != "privacy.officer" {
		t.Errorf("role details lost: %+v", room.Roles)
	}

	// A lifecycle label is authored or the operation's name, never derived.
	if got := room.Lifecycle.Archive.Label; got != "Archive the room" {
		t.Errorf("archive label = %q; want the authored label", got)
	}
	if got := room.Lifecycle.Reopen.Label; got != LifecycleReopen {
		t.Errorf("reopen label = %q; want the operation name", got)
	}

	su := room.MessageTypes["status-update"]
	if su.Persist != PersistLog || su.When == nil || su.Rate != l.LogRate {
		t.Errorf("status-update = %+v", su)
	}
	if p := su.Properties["severity"]; p.Type != PropertyEnum || p.Enum != "severity" ||
		!slices.Equal(p.Values, []string{"low", "medium", "high"}) {

		t.Errorf("severity property = %+v", p)
	}
	if lg := su.Actions["log"]; lg == nil || lg.PromoteTo == nil || lg.PromoteTo.Relate != "about" ||
		lg.PromoteTo.Map["title"] != "summary" || lg.Roles[0] != "lead" {

		t.Errorf("log action = %+v", lg)
	}
	if a := su.Actions["accept"]; a == nil || a.Permission != "incident.decide" || a.PromoteTo.Type != "decision" {
		t.Errorf("accept action = %+v", a)
	}
	typing := room.MessageTypes["typing"]
	if typing.Persist != PersistEphemeral || typing.TTL != 5*time.Second || typing.Rate.Count != 1 {
		t.Errorf("typing = %+v", typing)
	}
	if room.MessageTypes["text"].Properties["body"].Max != 8000 {
		t.Errorf("body max lost")
	}

	lc := room.Lifecycle
	if lc.Close == nil || len(lc.Close.Conditions) != 2 || len(lc.Close.Auto.When) != 2 {
		t.Errorf("close = %+v", lc.Close)
	}
	if !lc.Close.RequireAnchorRead || lc.Reopen.RequireAnchorRead {
		t.Errorf("RequireAnchorRead: close (with a condition) = %v, reopen (without) = %v; want true, false",
			lc.Close.RequireAnchorRead, lc.Reopen.RequireAnchorRead)
	}
	if got := lc.Close.Auto.When["datalek"].Fields; !slices.Equal(got, []string{"status"}) {
		t.Errorf("auto.when fields = %v", got)
	}
	if lc.Archive.From != StateClosed {
		t.Errorf("archive from = %q", lc.Archive.From)
	}
	if d, fixed := lc.Archive.Auto.After.Fixed(); !fixed || d != 30*24*time.Hour {
		t.Errorf("archive after = %v", lc.Archive.Auto.After)
	}
	if lc.Delete.Retention.String() != "7y" {
		t.Errorf("retention = %q", lc.Delete.Retention)
	}
	ops := room.MessageOps
	if ops.EditOwn.Within.String() != "15m" || ops.DeleteOwn.From != StateOpen || !ops.Remove.ReasonRequired {
		t.Errorf("message ops = %+v", ops)
	}

	threads, _ := c.ChannelType("page-threads")
	if !threads.Typing || room.Typing {
		t.Errorf("typing: page-threads = %v, incident-room = %v; want true, false", threads.Typing, room.Typing)
	}
	if threads.Anchor != AnchorFace || threads.Create == nil || threads.Roles["starter"].Source.Kind != SourceCreator {
		t.Errorf("page-threads = %+v", threads)
	}
	if threads.Read.Source.Kind != SourceAttached || !threads.Read.RequireAnchorRead {
		t.Errorf("default read = %+v; want attached, requiring anchor read", threads.Read)
	}
	if d, _ := threads.DetachedRetention.Fixed(); d != realtimelimits.DetachedRetention {
		t.Errorf("default detached retention = %v; want %v", threads.DetachedRetention, realtimelimits.DetachedRetention)
	}
	if room.DetachedRetention.String() != "90d" {
		t.Errorf("detached retention = %q; want 90d", room.DetachedRetention)
	}

	trs := c.Triggers()
	if len(trs) != 1 {
		t.Fatalf("triggers = %d", len(trs))
	}
	tr := trs[0]
	if tr.On != TriggerEnter || tr.Channel != "incident-room" || tr.PostType != "status-update" ||
		tr.Props["summary"] != "Severity raised to high" {

		t.Errorf("trigger = %+v", tr)
	}
	if !slices.Equal(tr.When.Fields, []string{"due", "severity"}) {
		t.Errorf("trigger fields = %v", tr.When.Fields)
	}
	if tr.WhenClosed != WhenClosedSkip || !strings.Contains(tr.When.Source, "days_between") {
		t.Errorf("trigger when_closed = %q, source = %q", tr.WhenClosed, tr.When.Source)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings = %v", w)
	}

	want := []string{"incident.decide", "privacy.officer", "records.delete"}
	if got := c.UsedPermissions(); !slices.Equal(got, want) {
		t.Errorf("UsedPermissions = %v; want %v", got, want)
	}
}

func TestCompiled_NilIsRealtimeOff(t *testing.T) {
	var c *Compiled
	if c.ChannelTypes() != nil || c.Triggers() != nil || c.UsedPermissions() != nil {
		t.Error("nil Compiled returned data")
	}
	if _, ok := c.ChannelType("x"); ok {
		t.Error("nil Compiled has a channel type")
	}
	if c.Limits().PageLimit != realtimelimits.MaxPageLimit {
		t.Error("nil Compiled does not report the built-in limits")
	}
}

// replace is one edit to the golden file.
type replace struct{ old, new string }

// TestParse_Invalid derives one invalid file per rule from the golden one
// and asserts the file, the key path and the reason of the refusal.
func TestParse_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"version", []replace{{"version: 1", "version: 2"}}, "version", "must be 1"},
		{"unknown key", []replace{{"on_entity_delete: block", "on_entity_delete: block\n    color: red"}},
			"channel_types.incident-room.color", "unknown key"},
		{"merge key in a map", []replace{{"      agent:  { principal_type: agent }\n",
			"      agent:  { principal_type: agent }\n      <<: { auditor: { permission: privacy.officer } }\n"}},
			"channel_types.incident-room.roles", "merge keys (<<) are not supported in realtime.yaml"},
		{"merge key in a struct", []replace{{"    cardinality: one\n    read:", "    cardinality: one\n    <<: { label: X }\n    read:"}},
			"channel_types.incident-room", "merge keys (<<) are not supported in realtime.yaml"},
		{"merge key at the top", []replace{{"channel_types:\n", "channel_types:\n  <<: { other: { attach_to: [datalek] } }\n"}},
			"channel_types", "merge keys (<<) are not supported in realtime.yaml"},
		{"state outputs", []replace{{"          label: Detected\n", "          label: Detected\n          outputs: []\n"}},
			"channel_types.breach-response.session.states.detected.outputs", "outputs belong to a transition"},
		{"reminder outputs", []replace{{"          label: Status update\n", "          label: Status update\n          outputs: []\n"}},
			"channel_types.breach-response.session.reminders.status-update.outputs", "a reminder posts a message only"},
		{"transition run_as", []replace{{"              roles: [dpo]\n              outputs:\n                - set: { properties: { status: notified, reference: \"{{message.reference}}\" } }\n          after:",
			"              roles: [dpo]\n              run_as: system:bot\n              outputs:\n                - set: { properties: { status: notified, reference: \"{{message.reference}}\" } }\n          after:"}},
			"channel_types.breach-response.session.states.assessed.on.notified.run_as",
			"run_as is set once per session type, at session.run_as"},
		{"state on_final", []replace{{"          label: Detected\n", "          label: Detected\n          on_final: close\n"}},
			"channel_types.breach-response.session.states.detected.on_final", "on_final belongs to the session"},
		{"list in a trigger literal", []replace{{"summary:  { type: string, required: true }",
			"summary:  { type: string, list: true, required: true }"}},
			"triggers.breach-escalated.post.props.summary", "a trigger or reminder cannot set a list property"},
		{"list in promote_to.map", []replace{{"summary:  { type: string, required: true }",
			"summary:  { type: string, list: true, required: true }"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.map.title",
			"message property summary (list of string) cannot fill timeline-entry.title"},
		{"session", []replace{{"version: 1\n", "version: 1\nsession: {}\n"}}, "session",
			"a session belongs under a channel type: channel_types.<name>.session"},
		{"session timers", []replace{{"      initial: detected\n", "      initial: detected\n      timers: {}\n"}},
			"channel_types.breach-response.session.timers", "reminders are declared under session.reminders"},
		{"state reminders", []replace{{"          label: Detected\n", "          label: Detected\n          reminders: {}\n"}},
			"channel_types.breach-response.session.states.detected.reminders",
			"reminders are declared once, under session.reminders, with in:"},
		{"reminder to", []replace{{"          max: 48\n", "          max: 48\n          to: overdue\n"}},
			"channel_types.breach-response.session.reminders.status-update.to",
			"a reminder posts a message and does not move the session"},
		{"reminder period", []replace{{"          max: 48\n", "          max: 48\n          period: 1h\n"}},
			"channel_types.breach-response.session.reminders.status-update.period", "use every"},
		{"reminder since", []replace{{"          max: 48\n", "          max: 48\n          since: session\n"}},
			"channel_types.breach-response.session.reminders.status-update.since",
			"a reminder counts from when the session entered one of its in: states"},
		{"agents", []replace{{"version: 1\n", "version: 1\nagents: {principal_types: [agent]}\n"}},
			"agents", "client_baselines"},
		{"action by", []replace{{"permission: incident.decide,", "permission: incident.decide, by: human,"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.by", "client_baselines"},
		{"list shape", []replace{{"attach_to: [datalek, incident]", "attach_to: datalek"}},
			"channel_types.incident-room.attach_to", "must be a list"},
		{"channel type name", []replace{{"  page-threads:", "  Page_Threads:"}},
			"channel_types.Page_Threads", "must match"},
		{"attach_to unknown", []replace{{"attach_to: [datalek, incident]", "attach_to: [datalek, nope]"}},
			"channel_types.incident-room.attach_to[1]", "unknown entity type"},
		{"attach_to alias", []replace{{"attach_to: [datalek, incident]", "attach_to: [breach, incident]"}},
			"channel_types.incident-room.attach_to[0]", "alias"},
		{"attach_to duplicate", []replace{{"attach_to: [datalek, incident]", "attach_to: [datalek, datalek]"}},
			"channel_types.incident-room.attach_to[1]", "twice"},
		{"attach_to empty", []replace{{"attach_to: [page]", "attach_to: []"}},
			"channel_types.page-threads.attach_to", "at least one"},
		{"anchor face without faces", []replace{{"cardinality: one\n", "cardinality: one\n    anchor: face\n"}},
			"channel_types.incident-room.anchor", "declares none"},
		{"anchor value", []replace{{"anchor: face", "anchor: entity"}},
			"channel_types.page-threads.anchor", "family or face"},
		{"cardinality value", []replace{{"cardinality: one", "cardinality: two"}},
			"channel_types.incident-room.cardinality", "one or many"},
		{"many without create", []replace{{"    create: { label: New thread, roles: [editor] }\n", ""}},
			"channel_types.page-threads.create", "needs create.roles"},
		{"one with create", []replace{{"cardinality: one\n", "cardinality: one\n    create: { roles: [lead] }\n"}},
			"channel_types.incident-room.create", "only a cardinality many"},
		{"create unknown role", []replace{{"roles: [editor] }", "roles: [boss] }"}},
			"channel_types.page-threads.create.roles[0]", "unknown role"},
		{"role two sources", []replace{{"lead:   { from: entity.owner,", "lead:   { from: entity.owner, permission: privacy.officer,"}},
			"channel_types.incident-room.roles.lead", "exactly one"},
		{"role no source", []replace{{"agent:  { principal_type: agent }", "agent:  { label: x }"}},
			"channel_types.incident-room.roles.agent", "exactly one"},
		{"role name", []replace{{"member: { from: attached }\n      agent:", "Member: { from: attached }\n      agent:"}},
			"channel_types.incident-room.roles.Member", "must match"},
		{"from syntax", []replace{{"from: entity.owner,", "from: owner,"}},
			"channel_types.incident-room.roles.lead.from", "attached or entity.<property>"},
		{"from missing property", []replace{{"from: entity.owner,", "from: entity.nope,"}},
			"channel_types.incident-room.roles.lead.from", "has no property"},
		{"from non-string property", []replace{{"from: entity.owner,", "from: entity.severity,"}},
			"channel_types.incident-room.roles.lead.from", "only a string property"},
		{"from restricted property", []replace{
			{"attach_to: [page]\n    anchor: face", "attach_to: [risico]"},
			{"starter: { creator: true }", "starter: { from: entity.owner }"},
		}, "channel_types.page-threads.roles.starter.from", "visible: grant"},
		{"creator on cardinality one", []replace{{"agent:  { principal_type: agent }", "agent:  { creator: true }"}},
			"channel_types.incident-room.roles.agent.creator", "cardinality many"},
		{"permission nobody holds", []replace{{"permission: privacy.officer, label", "permission: privacy.ofice, label"}},
			"channel_types.incident-room.roles.dpo.permission", "no role in acl.yaml grants"},
		{"post unknown role", []replace{{"post: [lead]\n", "post: [chief]\n"}},
			"channel_types.incident-room.message_types.status-update.post[0]", "unknown role"},
		{"message type name", []replace{{"typing: { persist", "Typing: { persist"}},
			"channel_types.incident-room.message_types.Typing", "must match"},
		{"no message types", []replace{{
			"message_types:\n      text:\n        properties: { body: { type: string, required: true } }\n" +
				"        post: [editor, starter]", "message_types: {}",
		}}, "channel_types.page-threads.message_types", "at least one message type"},
		{"persist value", []replace{{"persist: log", "persist: graph"}},
			"channel_types.incident-room.message_types.status-update.persist", "log or ephemeral"},
		{"ttl on log", []replace{{"label: Message\n", "label: Message\n        ttl: 5s\n"}},
			"channel_types.incident-room.message_types.text.ttl", "only an ephemeral"},
		{"ttl too long", []replace{{"ttl: 5s", "ttl: 2m"}},
			"channel_types.incident-room.message_types.typing.ttl", "at most 1m0s"},
		{"ttl calendar", []replace{{"ttl: 5s", "ttl: 1mo"}},
			"channel_types.incident-room.message_types.typing.ttl", "at most"},
		{"ttl syntax", []replace{{"ttl: 5s", "ttl: soon"}},
			"channel_types.incident-room.message_types.typing.ttl", "count from 1 to 100000"},
		{"rate above built-in", []replace{{"rate: 1/s", "rate: 5/s"}},
			"channel_types.incident-room.message_types.typing.rate", "above the limit"},
		{"rate above lowered limit", []replace{{"label: Message\n", "label: Message\n        rate: 25/m\n"}},
			"channel_types.incident-room.message_types.text.rate", "above the limit of 20/m"},
		{"property type", []replace{{"summary:  { type: string", "summary:  { type: strin"}},
			"channel_types.incident-room.message_types.status-update.properties.summary.type", "unknown type"},
		{"property without type", []replace{{"body: { type: string, required: true }", "body: { required: true }"}},
			"channel_types.page-threads.message_types.text.properties.body.type", "name a type"},
		{"property name", []replace{{"body: { type: markdown, required: true, max", "Body: { type: markdown, required: true, max"}},
			"channel_types.incident-room.message_types.text.properties.Body", "must match"},
		{"max on integer", []replace{{"affects:  { type: ref, to: [maatregel, risico] }", "affects:  { type: integer, max: 3 }"}},
			"channel_types.incident-room.message_types.status-update.properties.affects.max", "only a string or markdown"},
		{"max too large", []replace{{"max: 8000", "max: 20000"}},
			"channel_types.incident-room.message_types.text.properties.body.max", "between 1 and 16384"},
		{"ref without to", []replace{{"{ type: ref, to: [maatregel, risico] }", "{ type: ref }"}},
			"channel_types.incident-room.message_types.status-update.properties.affects.to", "names the entity types"},
		{"to on string", []replace{{"summary:  { type: string, required: true }", "summary:  { type: string, required: true, to: [risico] }"}},
			"channel_types.incident-room.message_types.status-update.properties.summary.to", "only a ref"},
		{"ref to unknown", []replace{{"to: [maatregel, risico]", "to: [maatregel, nope]"}},
			"channel_types.incident-room.message_types.status-update.properties.affects.to[1]", "unknown entity type"},
		{"message when", []replace{{"message.severity ~= ''", "message.nope == 'x'"}},
			"channel_types.incident-room.message_types.status-update.when", "does not compile"},
		{"post empty", []replace{{"rate: 1/s, post: [lead, dpo, member, agent]", "rate: 1/s, post: []"}},
			"channel_types.incident-room.message_types.typing.post", "name the roles"},
		{"ephemeral promote", []replace{{"rate: 1/s,", "rate: 1/s, promote_to: { type: decision },"}},
			"channel_types.incident-room.message_types.typing.promote_to", "cannot be promoted"},
		{"ephemeral actions", []replace{{"rate: 1/s,", "rate: 1/s, actions: { ok: { roles: [lead] } },"}},
			"channel_types.incident-room.message_types.typing.actions", "cannot carry actions"},
		{"promote type", []replace{{"promote_to: { type: timeline-entry", "promote_to: { type: nope"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.type", "unknown entity type"},
		{"promote entity property", []replace{{"map: { title: summary }", "map: { headline: summary }"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.map.headline", "has no property"},
		{"promote message property", []replace{{"map: { title: summary }", "map: { title: nope }"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.map.title", "has no property"},
		{"relate unknown", []replace{{"title: summary }, relate: { type: about }", "title: summary }, relate: { type: nope }"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.relate.type", "cannot link"},
		{"relate wrong endpoint", []replace{{"promote_to: { type: timeline-entry, map", "promote_to: { type: decision, map"}},
			"channel_types.incident-room.message_types.status-update.actions.log.promote_to.relate.type", "cannot link decision"},
		{"action without gate", []replace{{"accept: { label: Accept, permission: incident.decide,", "accept: { label: Accept,"}},
			"channel_types.incident-room.message_types.status-update.actions.accept", "roles or a permission"},
		{"action name", []replace{{"accept: { label", "Accept: { label"}},
			"channel_types.incident-room.message_types.status-update.actions.Accept", "must match"},
		{"action name reserved", []replace{{"accept: { label", "edit: { label"}},
			"channel_types.incident-room.message_types.status-update.actions.edit", "reserved"},
		{"lifecycle without gate", []replace{{"reopen:  { roles: [lead, dpo] }", "reopen:  { denied: no }"}},
			"channel_types.incident-room.lifecycle.reopen", "roles or a permission"},
		{"lifecycle from", []replace{{"from: closed, auto", "from: shut, auto"}},
			"channel_types.incident-room.lifecycle.archive.from", "open, closed or archived"},
		{"lifecycle from target", []replace{{"from: closed, auto", "from: archived, auto"}},
			"channel_types.incident-room.lifecycle.archive.from", "state it moves to"},
		{"condition", []replace{{`condition: "entity.status == 'closed'"`, `condition: "entity.nope == 'x'"`}},
			"channel_types.incident-room.lifecycle.close.condition", "does not compile for datalek"},
		{"condition restricted field", []replace{{`condition: "entity.status == 'closed'"`, `condition: "entity.notes == 'x'"`}},
			"channel_types.incident-room.lifecycle.close.condition", "visible: grant in acl.yaml restricts"},
		{"condition traversal", []replace{{`condition: "entity.status == 'closed'"`, `condition: "related(entity, 'about')"`}},
			"channel_types.incident-room.lifecycle.close.condition", "may not follow relations"},
		{"auto on reopen", []replace{{"reopen:  { roles: [lead, dpo] }", "reopen:  { roles: [lead, dpo], auto: { after: 1d } }"}},
			"channel_types.incident-room.lifecycle.reopen.auto", "only close and archive"},
		{"auto empty", []replace{{"auto: { after: 30d }", "auto: {}"}},
			"channel_types.incident-room.lifecycle.archive.auto", "when, after or both"},
		{"retention not on delete", []replace{{"from: closed, auto: { after: 30d }", "from: closed, retention: 1y, auto: { after: 30d }"}},
			"channel_types.incident-room.lifecycle.archive.retention", "only delete"},
		{"period syntax", []replace{{"within: 15m", "within: 15w"}},
			"channel_types.incident-room.message_ops.edit_own.within", "count from 1 to 100000"},
		{"auto when restricted field", []replace{{`auto: { when: "entity.status == 'closed'" }`, `auto: { when: "entity.notes == 'x'" }`}},
			"channel_types.incident-room.lifecycle.close.auto.when", "visible: grant in acl.yaml restricts"},
		{"auto when traversal", []replace{{`auto: { when: "entity.status == 'closed'" }`, `auto: { when: "related(entity, 'about')" }`}},
			"channel_types.incident-room.lifecycle.close.auto.when", "may not follow relations"},
		{"auto when whole entity", []replace{{`auto: { when: "entity.status == 'closed'" }`, `auto: { when: "related(entity, 'about')" }`}},
			"channel_types.incident-room.lifecycle.close.auto.when", "only as entity.<property>"},
		{"message op unknown role", []replace{{"remove:     { roles: [lead]", "remove:     { roles: [boss]"}},
			"channel_types.incident-room.message_ops.remove.roles[0]", "unknown role"},
		{"message op without roles", []replace{{"remove:     { roles: [lead], reason_required: true }", "remove:     { reason_required: true }"}},
			"channel_types.incident-room.message_ops.remove", "roles or a permission"},
		{"on_entity_delete", []replace{{"on_entity_delete: block", "on_entity_delete: cascade"}},
			"channel_types.incident-room.on_entity_delete", "delete, detach or block"},
		{"trigger name", []replace{{"breach-escalated:", "Breach:"}}, "triggers.Breach", "must match"},
		{"trigger on", []replace{{"on: enter", "on: always"}}, "triggers.breach-escalated.on", "enter, leave or change"},
		{"trigger entity type", []replace{{"entity_type: datalek", "entity_type: nope"}},
			"triggers.breach-escalated.entity_type", "unknown entity type"},
		{"trigger without when", []replace{{"    when: \"entity.severity == 'high' and days_between(entity.due, today()) < 3\"\n", ""}},
			"triggers.breach-escalated.when", "needs a when"},
		{"trigger function", []replace{{"entity.severity == 'high' and", "match(entity.status, 'c*') and"}},
			"triggers.breach-escalated.when", "may not call match"},
		{"trigger restricted field", []replace{{"entity.severity == 'high' and", "entity.notes == 'x' and"}},
			"triggers.breach-escalated.when", "reads datalek.notes"},
		{"trigger channel unknown", []replace{{"channel: incident-room", "channel: nope"}},
			"triggers.breach-escalated.channel", "unknown channel type"},
		{"trigger channel many", []replace{{"channel: incident-room", "channel: page-threads"}},
			"triggers.breach-escalated.channel", "cardinality one"},
		{"trigger channel not on entity", []replace{{"attach_to: [datalek, incident]", "attach_to: [incident]"}},
			"triggers.breach-escalated.channel", "does not attach to datalek"},
		{"trigger post type", []replace{{"post: { type: status-update", "post: { type: nope"}},
			"triggers.breach-escalated.post.type", "no message type"},
		{"trigger prop unknown", []replace{{"severity: high }", "severity: high, color: red }"}},
			"triggers.breach-escalated.post.props.color", "no property"},
		{"trigger prop substitution", []replace{{`summary: "Severity raised to high"`, `summary: "{{entity.title}} raised"`}},
			"triggers.breach-escalated.post.props.summary", "substitution is not supported"},
		{"trigger prop enum value", []replace{{"severity: high }", "severity: extreme }"}},
			"triggers.breach-escalated.post.props.severity", `"extreme" is not valid`},
		{"trigger prop ref", []replace{{"severity: high }", "severity: high, affects: MEA-1 }"}},
			"triggers.breach-escalated.post.props.affects", "only string, markdown or enum"},
		{"trigger required prop", []replace{{`summary: "Severity raised to high", `, ""}},
			"triggers.breach-escalated.post.props", `required property "summary"`},
		{"limit above built-in", []replace{{"post_rate: 20/m", "post_rate: 40/m"}}, "limits.post_rate", "may only lower"},
		{"limit syntax", []replace{{"post_rate: 20/m", "post_rate: fast"}}, "limits.post_rate", `must look like "30/m"`},
		{"limit burst", []replace{{"post_rate: 20/m", "post_rate: 1000/h"}}, "limits.post_rate", "in one burst"},
		{"signal_rate above built-in", []replace{{"page_limit: 100", "page_limit: 100\n  signal_rate: 5/s"}},
			"limits.signal_rate", "may only lower"},
		{"message_bytes zero", []replace{{"page_limit: 100", "page_limit: 100\n  message_bytes: 0"}},
			"limits.message_bytes", "between 1 and 16384"},
		{"anchor face partly faced", []replace{{"attach_to: [page]", "attach_to: [page, risico]"}},
			"channel_types.page-threads.anchor", `"risico" declares none`},
		{"delete_own from", []replace{{"from: open }", "from: gone }"}},
			"channel_types.incident-room.message_ops.delete_own.from", "open, closed or archived"},
		{"auto on delete", []replace{{"retention: 7y }", "retention: 7y, auto: { after: 1d } }"}},
			"channel_types.incident-room.lifecycle.delete.auto", "only close and archive"},
		{"read from restricted property", []replace{{"read: { from: attached, label", "read: { from: entity.notes, label"}},
			"channel_types.incident-room.read.from", "visible: grant"},
		{"trigger entity type alias", []replace{{"entity_type: datalek", "entity_type: breach"}},
			"triggers.breach-escalated.entity_type", `alias of "datalek"`},
		{"message-level promote", []replace{{"        actions:\n          log:",
			"        promote_to: { type: decision }\n        actions:\n          log:"}},
			"channel_types.incident-room.message_types.status-update.promote_to", "belongs on an action"},
		{"promote manual id", []replace{{"promote_to: { type: decision }", "promote_to: { type: handbook }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.type", "id_type: manual"},
		{"promote alias", []replace{{"promote_to: { type: decision }", "promote_to: { type: breach }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.type", "alias"},
		{"promote faced without face", []replace{{"promote_to: { type: decision }", "promote_to: { type: memo, map: { title: summary } }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.face", "name the face"},
		{"promote unknown face", []replace{{"promote_to: { type: decision }",
			"promote_to: { type: memo, face: final, map: { title: summary } }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.face", `no face "final"`},
		{"promote face on faceless type", []replace{{"promote_to: { type: decision }", "promote_to: { type: decision, face: draft }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.face", "declares no faces"},
		{"promote required unmapped", []replace{{"promote_to: { type: decision }", "promote_to: { type: memo, face: draft }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.map", `requires property "title"`},
		{"promote required from optional", []replace{{"promote_to: { type: decision }",
			"promote_to: { type: memo, face: draft, map: { title: severity } }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.map.title", "is optional"},
		{"promote type mismatch", []replace{{"promote_to: { type: decision }",
			"promote_to: { type: memo, face: draft, map: { title: summary, level: summary } }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.map.level", "cannot fill memo.level"},
		{"promote computed", []replace{{"promote_to: { type: decision }",
			"promote_to: { type: memo, face: draft, map: { title: summary, double: summary } }"}},
			"channel_types.incident-room.message_types.status-update.actions.accept.promote_to.map.double", "computed"},
		{"delete policy with retention", []replace{{"on_entity_delete: block", "on_entity_delete: delete"}},
			"channel_types.incident-room.on_entity_delete", "lifecycle.delete.retention must keep"},
		{"detached retention syntax", []replace{{"detached_retention: 90d", "detached_retention: soon"}},
			"channel_types.incident-room.detached_retention", "a count from 1"},
		{"detached retention below the orphan grace", []replace{{"detached_retention: 90d", "detached_retention: 30m"}},
			"channel_types.incident-room.detached_retention", "at least 1h0m0s"},
		{"limit int", []replace{{"page_limit: 100", "page_limit: 500"}}, "limits.page_limit", "between 1 and 200"},
		{"trigger ephemeral post type", []replace{{"post: { type: status-update, props: { summary: \"Severity raised to high\", severity: high } }",
			"post: { type: typing }"}}, "triggers.breach-escalated.post.type", "is ephemeral"},
		{"trigger props fail when", []replace{{`when: "message.severity ~= ''"`,
			`when: "message.summary == 'x'"`}}, "triggers.breach-escalated.post.props", "when: condition"},
		{"trigger message when on today", []replace{{`when: "message.severity ~= ''"`,
			`when: "message.severity ~= '' or today() == today()"`}}, "triggers.breach-escalated.post.props", "calls today()"},
		{"trigger prop over max", []replace{{"summary:  { type: string, required: true }",
			"summary:  { type: string, required: true, max: 5 }"}}, "triggers.breach-escalated.post.props.summary",
			"max of 5"},
		{"trigger prop over message_bytes", []replace{{"page_limit: 100", "page_limit: 100\n  message_bytes: 20"}},
			"triggers.breach-escalated.post.props", "over message_bytes"},
		{"trigger when_closed", []replace{{"channel: incident-room\n", "channel: incident-room\n    when_closed: drop\n"}},
			"triggers.breach-escalated.when_closed", "skip or post"},
	}
	meta, policy := testMeta(t), testPolicy(t)
	base := golden(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := base
			for _, e := range tc.edits {
				if strings.Count(src, e.old) != 1 {
					t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
				}
				src = strings.Replace(src, e.old, e.new, 1)
			}
			_, err := Parse([]byte(src), meta, policy)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

func assertProblem(t *testing.T, err error, key, reason string) {
	t.Helper()
	var cfgErr *Error
	if !errors.As(err, &cfgErr) {
		t.Fatalf("err = %v; want *realtimecfg.Error", err)
	}
	if !strings.HasPrefix(err.Error(), "realtime.yaml: ") {
		t.Errorf("error %q does not name realtime.yaml", err)
	}
	for _, p := range cfgErr.Problems {
		if p.Key == key && strings.Contains(p.Reason, reason) {
			return
		}
	}
	t.Errorf("no problem at %q containing %q; got:\n%s", key, reason, strings.ReplaceAll(err.Error(), "; ", "\n"))
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("version: [1"), testMeta(t), nil)
	assertProblem(t, err, "", "invalid YAML")
}

func TestParse_TooManyProperties(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\nchannel_types:\n  room:\n    attach_to: [datalek]\n    roles: {m: {from: attached}}\n" +
		"    message_types:\n      big:\n        post: [m]\n        properties:\n")
	for i := range realtimelimits.MaxMessageProperties + 1 {
		b.WriteString("          p" + strings.Repeat("x", i) + ": {type: string}\n")
	}
	_, err := Parse([]byte(b.String()), testMeta(t), nil)
	assertProblem(t, err, "channel_types.room.message_types.big.properties", "at most 32")
}

// Without acl.yaml there is no policy to check permissions or visible:
// grants against, so neither refuses.
func TestParse_NilPolicy(t *testing.T) {
	src := strings.Replace(golden(t), "permission: privacy.officer, label", "permission: anything, label", 1)
	src = strings.Replace(src, "entity.severity == 'high' and", "entity.notes == 'x' and", 1)
	c, err := Parse([]byte(src), testMeta(t), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !slices.Contains(c.UsedPermissions(), "anything") {
		t.Error("permission not collected without a policy")
	}
}

func TestParse_RequiresMeta(t *testing.T) {
	if _, err := Parse([]byte("version: 1"), nil, nil); err == nil {
		t.Fatal("Parse accepted a nil metamodel")
	}
}

func TestParse_Minimal(t *testing.T) {
	c, err := Parse([]byte("version: 1\n"), testMeta(t), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.ChannelTypes()) != 0 || c.Limits() != defaultLimits() {
		t.Errorf("minimal file compiled to %+v", c)
	}
}

func TestCheckReload(t *testing.T) {
	meta := testMeta(t)
	prev, err := Parse([]byte(golden(t)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rErr := CheckReload(prev, prev); rErr != nil {
		t.Errorf("identical reload refused: %v", rErr)
	}
	if rErr := CheckReload(nil, prev); rErr != nil {
		t.Errorf("first load refused: %v", rErr)
	}
	src := strings.Replace(golden(t), "    anchor: face\n", "", 1)
	next, err := Parse([]byte(src), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertProblem(t, CheckReload(prev, next), "channel_types.page-threads.anchor", "cannot change from face to family")
	// No valid edit of the golden file flips a cardinality alone (create,
	// creator and triggers all depend on it), so the parsed value is edited.
	next, err = Parse([]byte(golden(t)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	next.channelTypes["incident-room"].Cardinality = CardinalityMany
	assertProblem(t, CheckReload(prev, next), "channel_types.incident-room.cardinality", "cannot change from one to many")
	if _, pErr := ParsePeriod("100000y"); pErr != nil {
		t.Errorf("largest count refused: %v", pErr)
	}
	minimal, err := Parse([]byte("version: 1\n"), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckReload(prev, minimal); err != nil {
		t.Errorf("removing channel types refused: %v", err)
	}
}

// TestCheckReload_Refs pins that a ref property cannot turn into another
// type, while dropping it, its message type or its channel type is allowed.
func TestCheckReload_Refs(t *testing.T) {
	meta := testMeta(t)
	prev, err := Parse([]byte(golden(t)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	ct, _ := prev.ChannelType("incident-room")
	if got := ct.Shape().Refs; !slices.Equal(got, []string{"status-update.affects"}) {
		t.Fatalf("Shape().Refs = %v", got)
	}
	if !slices.Contains(ct.PlainProperties(), "status-update.summary") {
		t.Fatalf("PlainProperties() = %v", ct.PlainProperties())
	}
	const ref = "          affects:  { type: ref, to: [maatregel, risico] }\n"
	tests := []struct {
		name, with string
		refused    bool
	}{
		{"ref turned string", "          affects:  { type: string }\n", true},
		{"ref dropped", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(golden(t), ref, tc.with, 1)
			next, err := Parse([]byte(src), meta, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckReload(prev, next)
			if !tc.refused {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			assertProblem(t, err, "channel_types.incident-room.message_types.status-update.properties.affects.type",
				"cannot change from ref")
		})
	}
	stored := map[string]TypeShape{"incident-room": {
		Anchor: AnchorFamily, Cardinality: CardinalityOne, Refs: []string{"status-update.summary", "text.gone"},
	}}
	ms := ShapeMismatches(prev, stored)
	if len(ms) != 1 || !slices.Equal(ms[0].LostRefs, []string{"status-update.summary"}) {
		t.Fatalf("ShapeMismatches = %+v; want the summary ref lost and the undeclared one ignored", ms)
	}
}

func TestPeriod(t *testing.T) {
	start := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in    string
		fixed time.Duration
		isFix bool
		added time.Time
	}{
		{"30s", 30 * time.Second, true, start.Add(30 * time.Second)},
		{"15m", 15 * time.Minute, true, start.Add(15 * time.Minute)},
		{"2h", 2 * time.Hour, true, start.Add(2 * time.Hour)},
		{"7d", 7 * 24 * time.Hour, true, start.Add(7 * 24 * time.Hour)},
		{"1mo", 0, false, time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)},
		{"7y", 0, false, time.Date(2033, 1, 31, 12, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		p, err := ParsePeriod(tc.in)
		if err != nil {
			t.Fatalf("ParsePeriod(%q): %v", tc.in, err)
		}
		d, fixed := p.Fixed()
		if d != tc.fixed || fixed != tc.isFix || !p.AddTo(start).Equal(tc.added) || p.String() != tc.in || p.IsZero() {
			t.Errorf("%q: fixed=(%v,%v) added=%v str=%q", tc.in, d, fixed, p.AddTo(start), p)
		}
	}
	for _, bad := range []string{"", "m", "0m", "-1m", "5w", "99999999999999999999s", "100001d", "9223372036854775807h"} {
		if _, err := ParsePeriod(bad); err == nil {
			t.Errorf("ParsePeriod(%q) accepted", bad)
		}
	}
	if !(Period{}).IsZero() {
		t.Error("zero Period is not IsZero")
	}
}

// TestParse_FamilyFacedUnderFaceGrants pins D5: a trigger on a faced type
// that posts into a family-anchored channel is left out with a warning when
// acl.yaml admits readers through other faces, and the rest of the file
// stays in force. With anchor: face it loads.
func TestParse_FamilyFacedUnderFaceGrants(t *testing.T) {
	const src = `version: 1
channel_types:
  page-room:
    attach_to: [page]
    roles: { member: { from: attached } }
    message_types:
      note:
        properties: { body: { type: string } }
        post: [member]
triggers:
  page-owned:
    entity_type: page
    on: enter
    when: "entity.owner ~= ''"
    channel: page-room
    post: { type: note, props: { body: owned } }
`
	policy, err := acl.ParsePolicy([]byte("roles:\n  drafter:\n    read: [page@draft]\n"), "acl.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		src      string
		policy   *acl.Policy
		triggers int
		warn     bool
	}{
		{"family under face grants", src, policy, 0, true},
		{"family without face grants", src, nil, 1, false},
		{"face anchor", strings.Replace(src, "attach_to: [page]", "attach_to: [page]\n    anchor: face", 1), policy, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(tc.src), testMeta(t), tc.policy)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := len(c.Triggers()); got != tc.triggers {
				t.Errorf("triggers = %d; want %d", got, tc.triggers)
			}
			if _, ok := c.ChannelType("page-room"); !ok {
				t.Error("the channel type was dropped with the trigger")
			}
			w := c.Warnings()
			if tc.warn != (len(w) == 1) || (tc.warn && w[0].Key != "triggers.page-owned") {
				t.Errorf("warnings = %v", w)
			}
		})
	}
}

// TestParse_FamilyFacedAutoUnderFaceGrants pins D5 for auto.when: the rule
// is left out for a faced type under face grants, with a warning, and the
// operation and the rest of the file stay in force.
func TestParse_FamilyFacedAutoUnderFaceGrants(t *testing.T) {
	const src = `version: 1
channel_types:
  page-room:
    attach_to: [page]
    roles: { member: { from: attached } }
    message_types:
      note:
        properties: { body: { type: string } }
        post: [member]
    lifecycle:
      close: { roles: [member], auto: { when: "entity.owner ~= ''" } }
`
	policy, err := acl.ParsePolicy([]byte("roles:\n  drafter:\n    read: [page@draft]\n"), "acl.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		src    string
		policy *acl.Policy
		rules  int
		warn   bool
	}{
		{"family under face grants", src, policy, 0, true},
		{"family without face grants", src, nil, 1, false},
		{"face anchor", strings.Replace(src, "attach_to: [page]", "attach_to: [page]\n    anchor: face", 1), policy, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(tc.src), testMeta(t), tc.policy)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, ok := c.ChannelType("page-room")
			if !ok || ct.Lifecycle.Close == nil || ct.Lifecycle.Close.Auto == nil {
				t.Fatal("the channel type or its close was dropped with the rule")
			}
			if got := len(ct.Lifecycle.Close.Auto.When); got != tc.rules {
				t.Errorf("auto.when rules = %d; want %d", got, tc.rules)
			}
			w := c.Warnings()
			if tc.warn != (len(w) == 1) || (tc.warn && w[0].Key != "channel_types.page-room.lifecycle.close.auto.when") {
				t.Errorf("warnings = %v", w)
			}
		})
	}
}

func TestFacesRestricted(t *testing.T) {
	if facesRestricted(nil, "page") {
		t.Fatal("a nil policy restricted faces")
	}
	for _, tc := range []struct {
		name string
		role acl.RoleDef
		want bool
	}{
		{"a face-named read grant", acl.RoleDef{Read: []string{"page@draft"}}, true},
		{"a whole-type read grant", acl.RoleDef{Read: []string{"page", "*"}}, false},
		{"another type's face grant", acl.RoleDef{Read: []string{"memo@draft"}}, false},
		{"a world grant", acl.RoleDef{Worlds: []string{"published"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &acl.Policy{Roles: map[string]acl.RoleDef{"r": tc.role}}
			if got := facesRestricted(p, "page"); got != tc.want {
				t.Fatalf("facesRestricted(page) = %v; want %v", got, tc.want)
			}
		})
	}
}

const typingBase = `version: 1
channel_types:
  room:
    attach_to: [datalek]
    roles: { m: { from: attached } }
    message_types:
      text: { post: [m], properties: { body: { type: string } } }
`

// TestParse_Typing pins that typing: loads into ChannelType.Typing and that
// yaml.v3's boolean words are accepted as for every other boolean key.
func TestParse_Typing(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"on", "    typing: true\n", true},
		{"absent", "", false},
		{"off", "    typing: false\n", false},
		{"yaml word", "    typing: yes\n", true},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := strings.Replace(typingBase, "    message_types:\n", tc.line+"    message_types:\n", 1)
			c, err := Parse([]byte(src), meta, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("room")
			if ct.Typing != tc.want {
				t.Errorf("Typing = %v; want %v", ct.Typing, tc.want)
			}
		})
	}
}

// TestParse_TypingInvalid pins every load rule of typing: with its key path.
func TestParse_TypingInvalid(t *testing.T) {
	cases := []struct {
		name   string
		edits  []replace
		key    string
		reason string
	}{
		{"number", []replace{{"    message_types:\n", "    typing: 1\n    message_types:\n"}},
			"channel_types.room.typing", "must be true or false"},
		{"word", []replace{{"    message_types:\n", "    typing: maybe\n    message_types:\n"}},
			"channel_types.room.typing", "must be true or false"},
		{"mapping", []replace{{"    message_types:\n", "    typing: { on: true }\n    message_types:\n"}},
			"channel_types.room.typing", "must be a single value"},
		{"no log type", []replace{
			{"    message_types:\n", "    typing: true\n    message_types:\n"},
			{"text: { post: [m],", "text: { persist: ephemeral, ttl: 5s, post: [m],"},
		}, "channel_types.room.typing",
			"typing announces a message to come; declare a log message type or drop typing"},
		{"reserved name", []replace{
			{"    message_types:\n", "    typing: true\n    message_types:\n"},
			{"      text:", "      typing: { persist: ephemeral, ttl: 5s, post: [m] }\n      text:"},
		}, "channel_types.room.message_types.typing",
			`"typing" is reserved in a channel type with typing: true: the built-in indicator replaces it; ` +
				"remove this message type"},
		{"top level", []replace{{"version: 1\n", "version: 1\ntyping: true\n"}}, "typing", "unknown key"},
	}
	meta := testMeta(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := typingBase
			for _, e := range tc.edits {
				if strings.Count(src, e.old) != 1 {
					t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
				}
				src = strings.Replace(src, e.old, e.new, 1)
			}
			_, err := Parse([]byte(src), meta, nil)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestParse_BoolKeysUnchanged pins that the boolean check accepts exactly
// what yaml.v3 decodes: the YAML 1.1 words still load, and a value the
// decoder refuses is now refused with its key path.
func TestParse_BoolKeysUnchanged(t *testing.T) {
	meta := testMeta(t)
	src := golden(t)
	for _, e := range []replace{
		{"dismissed: { label: Dismissed, final: true,", "dismissed: { label: Dismissed, final: yes,"},
		{"body: { type: markdown, required: true, max: 8000 }", "body: { type: markdown, required: on, max: 8000 }"},
		{"reason_required: true", "reason_required: off"},
	} {
		if strings.Count(src, e.old) != 1 {
			t.Fatalf("edit %q matches %d times; want exactly one", e.old, strings.Count(src, e.old))
		}
		src = strings.Replace(src, e.old, e.new, 1)
	}
	c, err := Parse([]byte(src), meta, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	br, _ := c.ChannelType("breach-response")
	if st, ok := br.Session().State("dismissed"); !ok || !st.Final {
		t.Errorf("final: yes did not load as true: %+v", st)
	}
	room, _ := c.ChannelType("incident-room")
	if !room.MessageTypes["text"].Properties["body"].Required {
		t.Error("required: on did not load as true")
	}
	if room.MessageOps.Remove.ReasonRequired {
		t.Error("reason_required: off did not load as false")
	}

	bad := strings.Replace(golden(t), "starter: { creator: true }", "starter: { creator: 1 }", 1)
	_, err = Parse([]byte(bad), meta, nil)
	assertProblem(t, err, "channel_types.page-threads.roles.starter.creator", "must be true or false")
}

// TestParse_BranchListKeys pins the key walk of TKT-FPXTQJ AC 6: inside a
// list entry the walk names the index, a refused key keeps its reason, and
// a value that is neither a mapping nor a list is refused.
func TestParse_BranchListKeys(t *testing.T) {
	cases := []struct {
		name, on, key, reason string
	}{
		{"unknown key in an entry", "[ { to: working, nope: 1 } ]", branchKey + "[0].nope", "unknown key"},
		{"run_as in an entry", "[ { to: working, run_as: system:bot } ]", branchKey + "[0].run_as",
			"run_as is set once per session type, at session.run_as"},
		{"scalar", "working", branchKey, "must be a mapping or a list of mappings"},
		{"list of scalars", "[ working ]", branchKey + "[0]", "must be a mapping"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(branchSrc(tc.on)), testMeta(t), nil)
			assertOneProblem(t, err, tc.key, tc.reason)
		})
	}
}

// TestCheckReload_TypingToggleAllowed pins that typing does not decide how
// channels are keyed, so turning it on or off passes a reload.
func TestCheckReload_TypingToggleAllowed(t *testing.T) {
	meta := testMeta(t)
	on, err := Parse([]byte(golden(t)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	off, err := Parse([]byte(strings.Replace(golden(t), "    typing: true\n", "", 1)), meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ct, _ := off.ChannelType("page-threads"); ct.Typing {
		t.Fatal("typing still on after removing the key")
	}
	if err := CheckReload(on, off); err != nil {
		t.Errorf("turning typing off refused: %v", err)
	}
	if err := CheckReload(off, on); err != nil {
		t.Errorf("turning typing on refused: %v", err)
	}
}

// TestParse_ListProperty pins AC 2 of TKT-03FQPD: a list property loads
// with its limits and defaults, and each bad declaration is refused at its
// key.
func TestParse_ListProperty(t *testing.T) {
	src := func(prop string) string {
		return "version: 1\nchannel_types:\n  room:\n    attach_to: [datalek]\n    roles: {m: {from: attached}}\n" +
			"    message_types:\n      ask:\n        post: [m]\n        properties:\n          options: " + prop + "\n"
	}
	const k = "channel_types.room.message_types.ask.properties.options"
	loads := []struct {
		name          string
		prop          string
		max, maxItems int
	}{
		{"declared limits", "{ type: string, list: true, max: 80, max_items: 4 }", 80, 4},
		{"defaults", "{ type: string, list: true }", realtimelimits.DefaultListItemRunes, realtimelimits.MaxListItems},
		{"caps", "{ type: string, list: true, max: 500, max_items: 20 }", 500, 20},
	}
	for _, tc := range loads {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Parse([]byte(src(tc.prop)), testMeta(t), nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ct, _ := c.ChannelType("room")
			p := ct.MessageTypes["ask"].Properties["options"]
			if !p.List || p.Max != tc.max || p.MaxItems != tc.maxItems {
				t.Errorf("options = %+v; want a list with max %d and max_items %d", p, tc.max, tc.maxItems)
			}
		})
	}
	refused := []struct {
		name, prop, key, reason string
	}{
		{"list on markdown", "{ type: markdown, list: true }", k + ".list", "only a string property may be a list"},
		{"list on integer", "{ type: integer, list: true }", k + ".list", "only a string property may be a list"},
		{"list on an enum", "{ type: severity, list: true }", k + ".list", "only a string property may be a list"},
		{"max_items without list", "{ type: string, max_items: 3 }", k + ".max_items", "only a list property has max_items"},
		{"max_items zero", "{ type: string, list: true, max_items: 0 }", k + ".max_items", "must be between 1 and 20"},
		{"max_items over the cap", "{ type: string, list: true, max_items: 21 }", k + ".max_items",
			"must be between 1 and 20"},
		{"max over the cap", "{ type: string, list: true, max: 501 }", k + ".max", "must be between 1 and 500"},
		{"max zero", "{ type: string, list: true, max: 0 }", k + ".max", "must be between 1 and 500"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(src(tc.prop)), testMeta(t), nil)
			assertProblem(t, err, tc.key, tc.reason)
		})
	}
	t.Run("list on integer with max_items has one problem", func(t *testing.T) {
		_, err := Parse([]byte(src("{ type: integer, list: true, max_items: 3 }")), testMeta(t), nil)
		assertProblem(t, err, k+".list", "only a string property may be a list")
		if strings.Contains(err.Error(), k+".max_items") {
			t.Errorf("max_items got a problem although list: true is written: %v", err)
		}
	})
}
