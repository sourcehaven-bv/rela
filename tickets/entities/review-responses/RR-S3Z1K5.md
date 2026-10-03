---
id: RR-S3Z1K5
type: review-response
title: Unset WorldScope acts as trivial in tracer and search
finding: tracer.New, NewVisibleTracer, ResolveWorldPrimes, tracer.NodeOf, the raw search backends and lua.ReadDeps did not check IsSet (cranky review).
severity: significant
resolution: NewVisibleTracer rejects an unset world. ResolveWorldPrimes resolves nothing for one, which makes tracer.New and NodeOf fail closed. The linear, bleve and postgres Search methods return ErrInvalidQuery. lua.NewReader validation moves to PR 5a with the mail/script.go zero ReadDeps fix.
status: addressed
---
