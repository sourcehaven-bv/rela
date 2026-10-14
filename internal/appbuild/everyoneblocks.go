package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// warnEveryoneOpensFieldBlocks logs, at load, every field block the everyone
// role makes restrict nobody (BUG-Y5UF3K). `rela acl audit` reports the same
// as A14; this puts it in front of an operator who never runs the audit.
//
// It warns rather than refuses: the policy is valid and does what it says,
// and a deployment that opens a block to everyone on purpose must still boot.
//
// Nil: policy accepted (nothing to report); meta accepted (every type is
// treated as faceless).
func warnEveryoneOpensFieldBlocks(meta *metamodel.Metamodel, policy *acl.Policy) {
	facesOf := func(entityType string) []string { return metamodel.FaceOrderOf(meta, entityType) }
	for _, opened := range acl.EveryoneOpenedFieldBlocks(policy, facesOf) {
		slog.Warn("appbuild: the everyone role opens a field block", "detail", opened.Explain())
	}
}
