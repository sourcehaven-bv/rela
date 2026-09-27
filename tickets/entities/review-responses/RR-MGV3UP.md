---
id: RR-MGV3UP
type: review-response
title: Stdio defers snapshot build failure to each call
finding: attachmentDeps keeps the NewAttachmentSnapshot error and reports it per call.
severity: nit
reason: NewAttachmentSnapshot only fails on nil required deps, which a successfully assembled appbuild.Services cannot have; the error is still surfaced and logged per call rather than hidden. Returning it from deps() would ripple into the reload path for an impossible case.
status: wont-fix
---
