---
id: RR-ODPMN
type: review-response
title: validateCreateIDOpts returns string instead of error
finding: Returning a plain string and reserving "" as the success sentinel is unidiomatic Go. Two callers plug the string straight into writeV1Error/writeJSONError. Loses the ability to distinguish error kinds downstream and diverges from convention.
severity: minor
reason: Cosmetic. Function is private to the package, only two callers, both pipe the string verbatim into the HTTP error body. Switching to error or (errCode, message) would let the handlers branch on kind for finer-grained problem+json types — that's exactly the work tracked under RR-O1UMW (typed error codes). Bundling them lets the refactor of validateCreateIDOpts and the HTTP error layer happen as one coherent change rather than churning the signature twice.
status: deferred
---
