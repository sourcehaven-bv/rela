---
id: RR-VVPL
type: review-response
title: ErrEncrypted sentinel cannot expose path; Reason should be typed enum
finding: 'Plan: var ErrEncrypted = errors.New(...) wrapped via fmt.Errorf("%w: %s", ErrEncrypted, key). Works for errors.Is but consumers cannot extract the path from the error — they re-derive from filename. Cleaner: typed *EncryptedError struct with Path field, satisfying errors.Is(target, ErrEncrypted) via Is() method. Inaccessible.Reason field is a free-form string (''git-crypt encrypted'') — make it a typed enum constant (e.g. InaccessibleReasonGitCrypt) so the SPA can branch on a stable value not a localized string.'
severity: significant
resolution: Reason is now a typed enum (InaccessibleReason string with constant InaccessibleReasonGitCrypt). ErrEncrypted sentinel is no longer the surfacing mechanism — fsstore returns a populated entity, not an error. Path is implicit (the entity itself knows its ID and consumers know how to derive on-disk path from ID + type if needed).
status: addressed
---
