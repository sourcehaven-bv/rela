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

func TestACLCan_AllowExitsZero(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// PERS-ALICE is global security: can read incident.
	cmd := &ACLCanCmd{Principal: "PERS-ALICE", Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("allow must exit 0, got %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "ALLOW") {
		t.Errorf("expected ALLOW line, got: %s", got)
	}
}

func TestACLCan_DenyExitsNonZero(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatTable)

	// PERS-GHOST has no personal grant; deny → non-zero exit.
	cmd := &ACLCanCmd{Principal: "PERS-GHOST", Verb: "delete", Entity: "INC-042"}
	err := cmd.Run(context.Background(), svc)
	var exitErr *relaerrors.ExitError
	if !stderrors.As(err, &exitErr) {
		t.Fatalf("deny must return an ExitError, got %v", err)
	}
	if exitErr.Code == 0 {
		t.Errorf("deny exit code must be non-zero")
	}
	if got := buf.String(); !strings.Contains(got, "DENY") {
		t.Errorf("expected DENY line, got: %s", got)
	}
}

func TestACLCan_MissingEntityErrors(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	withOutput(t, output.FormatTable)

	cmd := &ACLCanCmd{Principal: "PERS-ALICE", Verb: "read", Entity: "NO-SUCH"}
	err := cmd.Run(context.Background(), svc)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing entity must error with 'not found', got %v", err)
	}
	// It must NOT be a plain deny ExitError.
	var exitErr *relaerrors.ExitError
	if stderrors.As(err, &exitErr) {
		t.Errorf("missing entity must not surface as a deny exit code")
	}
}

func TestACLCan_JSON(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), whoCanPolicy)
	seedWhoCanGraph(t, svc)
	buf := withOutput(t, output.FormatJSON)

	cmd := &ACLCanCmd{Principal: "PERS-ALICE", Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Fatalf("can json: %v", err)
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		Principal     string `json:"principal"`
		Allowed       bool   `json:"allowed"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if result.SchemaVersion != 1 || !result.Allowed || result.Principal != "PERS-ALICE" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestACLCan_NoPolicyAllows(t *testing.T) {
	svc := aclTestServices(t, whoCanMeta(), "") // no acl.yaml
	withOutput(t, output.FormatTable)

	cmd := &ACLCanCmd{Principal: "PERS-ALICE", Verb: "read", Entity: "INC-042"}
	if err := cmd.Run(context.Background(), svc); err != nil {
		t.Errorf("no policy must allow (exit 0), got %v", err)
	}
}
