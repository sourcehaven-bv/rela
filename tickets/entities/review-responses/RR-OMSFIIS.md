---
id: RR-OMSFIIS
type: review-response
title: Status and sync conflict lists hid config-file conflicts
finding: readableConflictPaths dropped every path that does not parse as an entity or relation conflict. Config files and modify/delete conflicts vanished from the status although the configuration is not a secret.
severity: significant
resolution: Only paths under the entity and relation directories are gated. Others are always listed. A data file without markers is judged by the row its path names. resolvedPath now resolves the nearest existing parent so a deleted file still counts as inside its directory. TestGitStatusResponse_ConflictFiles covers all cases.
status: addressed
---
