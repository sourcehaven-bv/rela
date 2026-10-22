---
id: RR-5E00YL
type: review-response
title: The file-tier crash window misattributes, which the Rekey window does not
finding: The plan writes the session audit record, then meta.yaml with the moved session, then the two log lines. lastSeq comes from the log, not meta.yaml. A crash after meta.yaml leaves the session moved and the audit record pointing at Seq n. The next append then stores a different message at seq n, so the audit trail names the wrong cause. A crash between the audit write and meta.yaml leaves a record of a transition that never happened, and a retry writes a second one. The Rekey window loses nothing and names nothing wrongly, so this is not the same class. Either write the session audit after the log lines (the trail may miss a record but never names a wrong cause), or state this exact failure in the package doc and put the cause's author and time in the record so a reader can detect the mismatch.
severity: minor
resolution: 'Plan updated: on the file tier the session audit is written after the log lines, so a crash can lose a record but never name a wrong cause; the package doc states the window.'
status: addressed
---
