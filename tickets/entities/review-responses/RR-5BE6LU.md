---
id: RR-5BE6LU
type: review-response
title: RecreateEntity nits
finding: unique.go comment disagreed with the e.ID argument; probe-before-authorize order undocumented; REQ-fromPeer test id; error prefixes used the bare id.
severity: nit
resolution: Comment explains why the id is passed; doc comment records the ordering; id renamed REQ-restored; both errors use FormatStateRef.
status: addressed
---
