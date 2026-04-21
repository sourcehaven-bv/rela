---
id: RR-E3IJ3
type: review-response
title: Error messages use 'storage:' prefix inconsistent with rest of package
finding: 'resolve errors use ''storage: ...'' strings while the rest of the storage package uses os.PathError or unwrapped strings. Wrapping with os.PathError would enable errors.Is(err, os.ErrInvalid) matching but leak the key in messages (mild info-leak).'
severity: nit
reason: 'The ''storage:'' prefix is consistent with the package-level error convention in other rela packages (state: prefix, workspace: prefix, etc.) and avoids the info-leak of echoing the potentially-attacker-controlled key in every error. The inconsistency within the storage package is real but minor, and a broader error-style normalization is its own ticket.'
status: wont-fix
---
