---
id: RR-TXV38
type: review-response
title: Audit at the CLI boundary via the audit.Audit sink, NOT a Manager.PurgeVersion method
finding: 'Both reviewers (cranky S1 = architect C1, the architect''s crux). ''Audit through the same path as writes'' must NOT become entitymanager.Manager.PurgeVersion: purge acts on a version ROW (neither entity nor relation), runs no automation/validation/ACL-write-gate, and Manager is under active god-object shrinking (TKT-N0IKN9) — a purge method is a foreign body there. The audit *path* is the audit.Audit SINK. Wire it: expose Audit() on cliServices/appbuild.Services (there is no svc.Audit() today — real gap), add an OpPurgeVersion audit op + a `version` Subject kind (audit.Subject has Kind + entity/relation identity but NO version/vseq/count field). The record carries: subject identity (id or triple), vseq(s)/count, op=purge-version, principal, optional --reason — NEVER the purged content (keep the audithook.go values-never-logged discipline; changedPropertyNames emits names only). Attribution: SystemUser()/ToolCLI does NOT identify the human operator — add a --reason/--operator flag (or resolve $USER/$SUDO_USER) folded into Summary so the one surviving record says WHO purged. Watch Summary length cap (filesystem.go truncateRunes): a --all purge records count+range, not an enumeration of every vseq.'
severity: significant
resolution: 'Design revised: audit via the audit.Audit sink wired onto cliServices/Services (not a Manager method); OpPurgeVersion + version Subject kind; required --reason in Summary; never echoes content; --all records count+range. See revised design #5.'
status: addressed
---
