---
id: RR-OW2QSH
type: review-response
title: Reason was a bare string with the CLI switching on a different field
finding: Two representations of one fact in different packages - a third reason would silently misrender
severity: significant
resolution: Reason is now a typed RelationFilenameReason with three constants; the CLI switches on it with a default arm rather than inferring the case from an empty FromContent. The constants are the JSON wire values.
status: addressed
---

`RelationFilenameIssue.Reason` carried two values as free prose, and the CLI did
not switch on it — it inferred the case from `iss.FromContent == ""`, an
implicit coupling to `checkRelationFile` leaving that field blank.

Adding a third reason (which the legacy-`type` finding did) would have silently
misclassified it as the unparseable-name case. The compiler could not help.

`Reason` is now a typed `RelationFilenameReason` with three constants, the CLI
switches on it with a `default` arm, and the string constants are the JSON wire
values.
