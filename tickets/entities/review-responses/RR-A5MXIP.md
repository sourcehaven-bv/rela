---
id: RR-A5MXIP
type: review-response
title: Ordered operators answered a list comparison with garbage instead of declining
finding: 'Leaving lt/lte/gt/gte comparing the stringified slice was described as ''stays scalar'' but actually compared the Go slice literal: filter[tags][lt]=zzz returned EVERY entity, since "[a b]" sorts below "zzz". Declining to define list-ordering semantics does not justify answering anyway with a confident wrong result — the same class as the original bug.'
severity: significant
resolution: An ordered operator against a list-valued property now matches no rows and logs a warning, via a new isListValue guard. This is the fail-closed direction consistent with the rest of the file, and leaves the semantics genuinely free to be defined later rather than accidentally defined as lexicographic-on-slice-literal. Scalar and null properties keep their existing comparison — this bug does not redefine those. Pinned by a test.
status: addressed
---
