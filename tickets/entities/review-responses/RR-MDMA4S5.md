---
id: RR-MDMA4S5
type: review-response
title: No parsers or text forms for Digest and Fingerprint
finding: There is no canonical.ParseDigest and no text form for Fingerprint; later steps that read stored hex hashes or exchange fingerprints with a peer would each invent one.
severity: minor
resolution: TKT-VP02WHG adds Fingerprint.String/ParseFingerprint/MarshalText/UnmarshalText (32 lower-case hex digits, Hi first) and Digest.MarshalText/UnmarshalText; the _reconcile wire uses them. Pinned by internal/reconcile/text_test.go.
reason: The wire encoding is the _reconcile endpoint's decision (step 5 of FEAT-CMVZQFI) and storage is TKT-1G2XR1R's (which computes from the row). Adding unused codecs now would fix a format before its consumer exists; this package already pins the binary layout through Limbs and the golden fingerprints.
status: addressed
---
