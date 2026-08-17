---
id: DOCS-BHFCLM
type: docs-checklist
title: 'Docs: Next-action layer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the config types (`NextActionBand`, `NextActionSource`,
      `NextActionOffer`, `NextActionPickOne`, the prominence and defer-scope
      enums), each carrying the *reason* for its shape rather than restating
      the field — why bands beat numeric priorities, why prominence is a closed
      vocabulary, why `defer_scope` cannot be inferred.
- [x] Package doc on `internal/nextaction` explaining what the one-slot
      constraint buys: no per-source `limit:`, no ranking within a band, no
      cache.
- [x] Package doc on `internal/userstate` stating why this state is not graph
      content, and on each backend stating its concurrency limits —
      `kvuserstate` names its whole-document clobber explicitly rather than
      understating it.
- [x] The two ACL-sensitive seams carry their reasoning at the call site:
      `queryCandidates` (field redaction, and why serialization-time redaction
      does not reach a suggestion) and `countCandidates` (gated by default,
      `count_ungated` never inferred).
- [x] ~~CLAUDE.md pattern update~~ (N/A: no new architectural pattern — the
      layer uses the existing consumer-side-interface, build-tagged-backend and
      conformance-suite patterns rather than introducing one.)

## Project Documentation

- [x] `docs/data-entry.md` — a `## Next actions` section covering the framing
      (advisory, one at a time, could-not-should), bands and prominence,
      sources and their fields, `key_props`, `defer_scope`, the read-gated
      `count`, the affordance vocabulary, and where per-user state lives.
- [x] A worked example: the four-source consultancy configuration from
      RES-09YLLL, with the suggestion sequence it produces and a note on why
      two near-identical rules sit in different bands.
- [x] Customisation documented against `docs/customisation.md`'s tiers — the
      `<rela-slot name="companion">` hook and the `rela-na*` classes, with a
      worked `custom.js` example.

## External Documentation

- [x] ~~README / external docs~~ (N/A: the feature is configured in
      `data-entry.yaml`, so `docs/data-entry.md` is its home.)

**Docs verified:** the worked example was run against a live server — every
suggestion in the documented sequence appears in the order shown, and the
`pick_one` limit behaves as described.
