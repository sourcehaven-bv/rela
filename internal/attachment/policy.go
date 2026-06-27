package attachment

import (
	"context"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// PolicyProcessor is the default attachment [Processor] for the data-entry and
// CLI write paths. It resolves the effective per-property attachment policy
// (MIME allowlist, scan, transforms) from the metamodel and applies the
// matching steps in order:
//
//	MIME allowlist (native) → scan (cmd:) → transforms (cmd:)
//
// In Phase 1 only the native MIME allowlist runs; the scan/transform steps are
// wired in Phase 2 via the [CommandRunner] seam. A PolicyProcessor with a nil
// runner does MIME validation only, which is the correct out-of-box behavior.
type PolicyProcessor struct {
	meta   *metamodel.Metamodel
	runner CommandRunner // optional; nil → no scan/transform (Phase 2 wires it)
}

// NewPolicyProcessor builds the policy processor. meta is required; runner is
// optional (nil disables scan/transform, leaving native MIME validation only).
func NewPolicyProcessor(meta *metamodel.Metamodel, runner CommandRunner) *PolicyProcessor {
	return &PolicyProcessor{meta: meta, runner: runner}
}

// NeedsFullFile always returns true: MIME sniffing, scanning and transforms all
// need the complete bytes.
func (p *PolicyProcessor) NeedsFullFile() bool { return true }

// Process applies the resolved policy steps for the property named in pc.
func (p *PolicyProcessor) Process(
	ctx context.Context, pc ProcessContext, r io.Reader,
) (io.Reader, ProcessInfo, error) {
	prop, ok := p.propertyDef(pc.EntityType, pc.Property)
	if !ok {
		// No metamodel entry — nothing to validate against; pass through.
		return r, ProcessInfo{}, nil
	}

	out := r
	info := ProcessInfo{}

	// 1. Native MIME allowlist (always on; default-safe preset when unset).
	allow := prop.Accept
	if len(allow) == 0 && p.meta != nil && p.meta.Attachments != nil {
		allow = p.meta.Attachments.Allow
	}
	mimeOut, mimeInfo, err := newMIMEProcessor(allow).Process(ctx, pc, out)
	if err != nil {
		return nil, ProcessInfo{}, err
	}
	out = mimeOut
	if mimeInfo.FileName != "" {
		info.FileName = mimeInfo.FileName
	}

	// 2 & 3. Scan + transforms (cmd:) — only when a runner is wired (Phase 2).
	if p.runner != nil {
		out, info, err = p.applyCommands(ctx, pc, prop, out, info)
		if err != nil {
			return nil, ProcessInfo{}, err
		}
	}

	return out, info, nil
}

// transformCommands returns the ordered transform commands for a property.
func (p *PolicyProcessor) transformCommands(prop metamodel.PropertyDef) [][]string {
	cmds := make([][]string, 0, len(prop.Transform))
	for _, step := range prop.Transform {
		if len(step.Cmd) > 0 {
			cmds = append(cmds, step.Cmd)
		}
	}
	return cmds
}

// propertyDef looks up a property definition by entity type and name.
func (p *PolicyProcessor) propertyDef(entityType, property string) (metamodel.PropertyDef, bool) {
	if p.meta == nil {
		return metamodel.PropertyDef{}, false
	}
	def, ok := p.meta.GetEntityDef(entityType)
	if !ok {
		return metamodel.PropertyDef{}, false
	}
	prop, ok := def.Properties[property]
	return prop, ok
}
