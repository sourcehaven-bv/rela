---
id: RR-XNPEAE
type: review-response
title: ZIP rejection tests assert only ErrRejected, so they cannot catch the regression they were written for
finding: 'Two independent mechanisms reject ZIP bytes: deniedExtensions (before sniffing, host-independent) and the sniff-vs-claim mismatch (resolves a claim through the OS MIME database). Every new test asserted only errors.Is(err, ErrRejected), which cannot distinguish them. Verified concretely: removing .jar from deniedExtensions left TestMIME_ZipToleranceIsNotBlanket PASSING, because on a populated host .jar falls through to the claim mismatch instead. On a minimal image that same change fails open with no test failing — which is exactly the RR-7C5D7T defect returning undetected. Five of the seven cases in a test named ZipToleranceIsNotBlanket also never reached the ZIP tolerance at all, being rejected five lines earlier at the extension deny check.'
severity: significant
resolution: 'Replaced by TestMIME_ZipRejectionsNameTheirMechanism, a table-driven test that asserts WHICH check fired by matching the error message — ''is not allowed'' for the deny set, ''file extension implies'' for the polyglot mismatch. Re-ran the same experiment: removing .jar from deniedExtensions now FAILS with ''rejected by the wrong check'', naming both the message it got and the one it wanted. The structural half (every executable/macro extension is in deniedExtensions) stays in TestMIME_ExecutableZipContainersDeniedByExtension, and TestMIME_ExtensionSetsAreDisjoint was added so an extension cannot be both tolerated and denied with the outcome decided by check ordering.'
status: addressed
---
