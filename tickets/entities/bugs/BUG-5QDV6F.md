---
id: BUG-5QDV6F
type: bug
title: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)
description: |-
    IB-review finding on PR #1114 (BUG-ZWTDH9), tracked as GitHub #1127.

    BUG-ZWTDH9 removed the "create-then-Update-on-ErrConflict" upsert idiom from internal/entitymanager because it enabled a cross-type authorization bypass. internal/rename/rename.go retains a package-local copy in upsertEntity (rename.go:209-218) and upsertRelation (rename.go:222-240): CreateEntity/CreateRelation, and on store.ErrConflict fall back to UpdateEntity/UpdateRelation.

    validatePreconditions (rename.go:125-136) does a non-atomic GetEntity(newID) 'is the target free?' check. On the multi-writer postgres backend a concurrent create of the same ID can land between that check and the write; the Create then returns ErrConflict and the helper silently UPDATEs (clobbers) the racing entity — a lost update.

    Unlike BUG-ZWTDH9 there is NO cross-type escalation here: the type is taken from oldEntity.Type (server-side, not client-controlled). It is the same lost-update/clobber bug class only.

    Verified scope: rename.go is the SOLE remaining site of the Create-then-Update-on-conflict fallback. Every other ErrConflict handler in entitymanager/dataentry/cli fails closed (returns an error), matching the sanctioned resolve-by-intent pattern in entitymanager/apply.go. The 'auto-magic upsert' fallback is semantically wrong for rename regardless of the race: validatePreconditions already requires newID to be free and each renamed relation key is fresh (self-ref handled by the writeRenamedRelations skip), so Create can only ever conflict on a race — and firing the Update fallback IS the bug. Fix: delete the upsert helpers, use strict Create, surface ErrConflict as ErrEntityAlreadyExists so a lost race fails loudly.

    Grondslag: POLICY-015 §3 (security risks recognised and controlled early). Flagged as a follow-up in REV-E546FQ.md but had no ticket/owner/timeline.
priority: medium
effort: s
why1: 'A rename can silently overwrite a concurrently-created entity/relation: upsertEntity/upsertRelation (rename.go:209-240) treat store.ErrConflict from Create as ''already exists, so Update instead'' and fall through to UpdateEntity/UpdateRelation. On a multi-writer store ErrConflict also means ''someone else just created a DIFFERENT row at this key''; the helper cannot distinguish the two and clobbers it (lost update).'
why2: The author treated the later Create as one that 'should always succeed' because validatePreconditions (rename.go:129) already GetEntity-checked that newID was free, so the Update fallback was intended as harmless belt-and-suspenders rather than a real second write path.
why3: 'That assumption is a TOCTOU: the free-ID probe and the write are two separate, non-atomic store calls. It held under the single-writer fsstore/memstore world, but the postgres multi-writer backend (cross-process writes via LISTEN/NOTIFY) removed the ''only one writer, the probe stays valid'' premise the fallback silently relied on.'
why4: 'The BUG-ZWTDH9 fix (PR #1114) removed the identical idiom but was scoped to internal/entitymanager (core.go, apply.go, manager.go, cascadehost.go), where the security-critical cross-type bypass lived. internal/rename is a separate package with a package-local copy; the reviewer flagged it in REV-E546FQ.md as an out-of-scope follow-up but no ticket/owner/timeline was created, so it was never closed.'
why5: 'Systemic: ''upsert = Create then Update-on-ErrConflict'' was an ambient idiom copy-pasted across write paths, with no single sanctioned create-XOR-update primitive and no lint/arch rule forbidding the non-atomic fallback. Duplication plus reliance on human review to find every copy meant fixing one instance could not guarantee the others, and a known follow-up had no tracking mechanism to force closure.'
prevention: 'P2 (implemented): re-routed Manager.RenameEntity through the atomic store.RenameEntity (single pgstore transaction; locked section on mem/fs) and DELETED the non-atomic internal/rename decompose-into-create+delete package entirely — retiring the lost-update/clobber/partial-failure class at the source rather than hardening one half of it. No fallback needed: RenameEntity is a mandatory store.Store method every backend implements. P4 (automated measure rename-no-overwrite-test): TestRename_TargetExistsDoesNotOverwrite (entitymanager) + storetest RenameEntity conformance across mem/fs/pg. P5 (systemic follow-up TKT-XFA7SC): CI/arch-lint rule forbidding the Create-then-Update-on-ErrConflict idiom from recurring anywhere. Note: with the decomposition gone from the rename path, the last create-then-update-on-conflict fallback in the tree is removed — TKT-XFA7SC now guards against reintroduction generally.'
status: done
---
