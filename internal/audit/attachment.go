package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Attachment write denials are recorded by every attachment ingress (the
// data-entry upload/delete routes and the MCP attachment tools). The record
// shapes live here, beside [ElevationRecorder], so the ingresses cannot drift
// apart: an operator filtering `op == "denied-write"` must see the same
// summary whichever surface refused the write.
//
// Both are [OpDeniedWrite] and carry `op=attachment-write` in the summary. The
// ACL denial and the policy rejection are the same op on purpose, so a filter
// catches both without knowing there are two (TKT-6O8D0L).

// AttachmentWriteDenied returns the record for an attachment write the ACL
// refused before any bytes were stored. reason, ruleKind and ruleID come from
// the ACL decision.
func AttachmentWriteDenied(ctx context.Context, entityType, entityID, reason, ruleKind, ruleID string) Record {
	return Record{
		Time:        time.Now().UTC(),
		Op:          OpDeniedWrite,
		Subject:     &Subject{Kind: "entity", Type: entityType, ID: entityID},
		Principal:   principal.From(ctx),
		TriggeredBy: TriggeredByFrom(ctx),
		Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=attachment-write)",
			reason, ruleKind, ruleID),
	}
}

// AttachmentRejected returns the record for an upload the attachment
// processor refused: a disallowed MIME type, or a failed or positive scan
// (TKT-6O8D0L, CONTROL-8-15). A refused upload may be an attempt to place a
// disallowed file or malware in the project, which is the forensic question
// the audit log exists to answer.
func AttachmentRejected(ctx context.Context, entityType, entityID, property, fileName, reason string) Record {
	return Record{
		Time:        time.Now().UTC(),
		Op:          OpDeniedWrite,
		Subject:     &Subject{Kind: "entity", Type: entityType, ID: entityID},
		Principal:   principal.From(ctx),
		TriggeredBy: TriggeredByFrom(ctx),
		Summary: fmt.Sprintf("rejected upload %q to property %q: %s (op=attachment-write)",
			fileName, property, reason),
	}
}
