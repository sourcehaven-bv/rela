---
id: RR-OMZL3B
type: review-response
title: CLI purge reports a commit-time refusal as success
finding: A refusal at commit returns Purged==0 with no error; the CLI prints success and writes an OpPurgeVersion audit record.
severity: significant
resolution: PurgeResult.Refusal set by both backends; the CLI errors on a commit refusal and writes no audit record. TestHistoryPurgeCmd_SQLiteRefusedAtCommit.
status: addressed
---
