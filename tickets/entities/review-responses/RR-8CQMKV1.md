---
id: RR-8CQMKV1
type: review-response
title: Zero Kind yields a fingerprint
finding: The zero Kind and zero Element still produce a valid-looking fingerprint.
severity: nit
resolution: Kinds are the bytes E and R. The zero Kind is documented as invalid and Fingerprint panics on an unknown kind since only programmer error builds one.
status: addressed
---
