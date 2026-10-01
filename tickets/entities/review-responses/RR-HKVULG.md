---
id: RR-HKVULG
type: review-response
title: 'PR 8: lua ReadDeps.World unset at three wiring sites'
finding: dataentry validator deps, docs runtime and mail script leave ReadDeps.World zero.
severity: minor
resolution: 'ReadDeps.World is required: NewReader panics when unset. SetWorlds rebuilds the validator in the default world (TestSetWorlds_RewiresValidatorWorld). The mail script uses a detached runtime with zero ReadDeps; the docs runtime gets an explicit world. The archguard worldfield guard flags any ReadDeps literal without World.'
reason: The validator deps are built in NewApp before SetWorlds supplies the lookup; the zero scope is the default world today. The docs runtime reads a throwaway memstore and the mail script has no store. Wiring the configured default belongs to TKT-7IZHP0.
status: addressed
---
