package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// canRelationPolicy grants PERS-ALICE the edge permission and nothing else on
// the source type; PERS-GHOST gets neither.
const canRelationPolicy = `
roles:
  edge-writer:
    read: ["*"]
    permissions: [link-editor]
assignments:
  PERS-ALICE: edge-writer
relation_grants:
  editor-of:
    create: link-editor
`

func TestACLCanRelation_AllowExitsZero(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), canRelationPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLCanRelationCmd{
		Principal: "PERS-ALICE", Verb: "create", Relation: "editor-of", From: "PERS-ALICE",
	}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("allow must exit 0, got %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "ALLOW") {
		t.Fatalf("expected ALLOW line, got: %s", got)
	}
	// The allow must say WHICH configuration produced it: revoking a
	// relation_grants permission and revoking a source-type verb grant are
	// different edits in different places.
	if !strings.Contains(got, "relation_grants") || !strings.Contains(got, "link-editor") {
		t.Errorf("allow does not attribute the grant: %s", got)
	}
}

func TestACLCanRelation_DenyExitsNonZeroAndNamesTheGate(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), canRelationPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	cmd := &ACLCanRelationCmd{
		Principal: "PERS-GHOST", Verb: "create", Relation: "editor-of", From: "PERS-ALICE",
	}
	err := cmd.Run(context.Background(), svc)
	var exitErr *relaerrors.ExitError
	if !stderrors.As(err, &exitErr) {
		t.Fatalf("deny must return an ExitError, got %v", err)
	}
	if exitErr.Code == 0 {
		t.Error("deny exit code must be non-zero — the command is a CI gate")
	}
	got := buf.String()
	if !strings.Contains(got, "DENY") {
		t.Fatalf("expected DENY line, got: %s", got)
	}
	if !strings.Contains(got, "rule_kind=") {
		t.Errorf("deny does not name the deciding gate; 'which gate said no' is "+
			"the question an operator actually has: %s", got)
	}
}

// TestACLCanRelation_UndeclaredRelationIsNotADeny pins that a typo'd relation
// type errors instead of reporting a considered "no". The gate would happily
// deny a nonexistent type, and that denial would read as a real answer.
func TestACLCanRelation_UndeclaredRelationIsNotADeny(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), canRelationPolicy)
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	cmd := &ACLCanRelationCmd{
		Principal: "PERS-ALICE", Verb: "create", Relation: "editor-offf", From: "PERS-ALICE",
	}
	err := cmd.Run(context.Background(), svc)
	if err == nil {
		t.Fatal("an undeclared relation type must error, not report DENY")
	}
	var exitErr *relaerrors.ExitError
	if stderrors.As(err, &exitErr) {
		t.Fatal("a typo must not surface as a plain deny exit")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Errorf("error %q does not explain the typo", err)
	}
}

func TestACLCanRelation_MissingSourceEntityErrors(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), canRelationPolicy)
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	cmd := &ACLCanRelationCmd{
		Principal: "PERS-ALICE", Verb: "create", Relation: "editor-of", From: "NO-SUCH",
	}
	err := cmd.Run(context.Background(), svc)
	if err == nil {
		t.Fatal("a missing source entity must error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q does not report the missing entity", err)
	}
}

func TestACLCanRelation_JSONShape(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), canRelationPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLCanRelationCmd{
		Principal: "PERS-ALICE", Verb: "create", Relation: "editor-of", From: "PERS-ALICE",
	}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got struct {
		SchemaVersion int    `json:"schema_version"`
		Verb          string `json:"verb"`
		Relation      string `json:"relation"`
		From          string `json:"from"`
		FromType      string `json:"from_type"`
		Allowed       bool   `json:"allowed"`
		RuleKind      string `json:"rule_kind"`
		RuleID        string `json:"rule_id"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON: %v (%s)", err, buf.String())
	}
	wrong := got.SchemaVersion == 0 || !got.Allowed ||
		got.Relation != "editor-of" || got.FromType != "person" ||
		got.RuleKind != "relation-grant" || got.RuleID != "link-editor"
	if wrong {
		t.Errorf("unexpected JSON payload: %+v", got)
	}
}

// TestACLCanRelation_NoPolicyIsAllowButStillGatesExistence mirrors `acl can`:
// with no acl.yaml everything is permitted, but a typo'd id must not exit
// green on nothing.
func TestACLCanRelation_NoPolicyIsAllowButStillGatesExistence(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), "")
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	ok := &ACLCanRelationCmd{
		Principal: "ANYONE", Verb: "create", Relation: "editor-of", From: "PERS-ALICE",
	}
	if err := ok.Run(context.Background(), svc); err != nil {
		t.Fatalf("no-policy must allow, got %v", err)
	}

	missing := &ACLCanRelationCmd{
		Principal: "ANYONE", Verb: "create", Relation: "editor-of", From: "NO-SUCH",
	}
	if err := missing.Run(context.Background(), svc); err == nil {
		t.Error("no-policy must still gate source-entity existence")
	}
}
