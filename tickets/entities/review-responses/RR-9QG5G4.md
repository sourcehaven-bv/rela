---
id: RR-9QG5G4
type: review-response
title: RefuseRunAsEnv's nil contract is wrong
finding: "internal/principal/principal.go:623 says \"Nil: lookup is rejected — the caller passes os.LookupEnv\". Nothing rejects a nil lookup; calling it panics.\tA reader expects an error for a nil lookup and gets a panic.\tWrite \"Nil: not accepted; a nil lookup panics\", or return an error for nil."
severity: nit
resolution: Nil contract now says a nil lookup is not accepted and panics.
reason: Nil contract now says a nil lookup is not accepted and panics.
status: addressed
---
