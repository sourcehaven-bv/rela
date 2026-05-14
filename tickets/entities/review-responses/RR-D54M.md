---
id: RR-D54M
type: review-response
title: Frontend ID allowlist is stricter than backend storeutil.ValidateID — picker can silently swallow valid IDs
finding: |-
    `insertEntityRef.ts` defines `ID_REGEX = /^[A-Za-z][A-Za-z0-9_-]{0,255}$/`. The backend `internal/store/storeutil/storeutil.go:ValidateID` is FAR more permissive: it rejects only empty IDs, `--`, path separators (`/`, `\`), and ASCII control characters. There is no leading-letter rule, no 256-char cap, and the character set is open.

    The picker calls `/_search` which returns whatever the store has. Result rows have `.id` straight from the store. If a future or already-existing manual-id entity has an ID like `2024-q1-review` or `iso-27001-a.5.1` (leading digit, contains a `.`), the picker will display it, the user clicks, and `insertEntityRef` silently no-ops because the ID fails the frontend regex. The picker closes, nothing is inserted, no feedback is given to the user. They will think the feature is broken.

    This is a leaky abstraction: the frontend invents its own ID grammar that does not match the source of truth. The 'defensive validation costs nothing' rationale in the code comment is exactly backwards — the cost is silently dropping the only action the picker exists to perform.

    Fix options, in decreasing strictness: (a) Re-derive the regex from the actual storeutil rule (reject only the dangerous chars, allow leading digits, allow `.`, drop the 256-char cap or raise it). (b) Drop the allowlist entirely — the only injection vector is backticks, newlines, and control chars; gate on those instead of a positive allowlist. (c) If keeping a strict allowlist, surface the rejection to the user via a toast (`'Cannot insert reference: invalid ID format'`) so silent failure becomes observable.

    The TKT-I5NO-specific risk is low TODAY because no entity in the repo has a digit-leading ID. But the picker is shipped against a metamodel-driven store, and the metamodel does not constrain IDs the way the frontend does. The first time a user authors a manual-id entity with a leading digit, the picker becomes a black hole.
severity: significant
resolution: 'Replaced the strict allowlist regex with a denylist that mirrors internal/store/storeutil.ValidateID: rejects empty, ''--'', path separators, ASCII control chars, plus the code-span-specific backtick and whitespace. Cap raised to 1024 bytes. Now accepts the IDs the backend accepts: leading digits (''2024-q1''), dots (''iso-27001-a.5.1''), longer manual IDs. New test asserts these previously-rejected shapes pass.'
status: addressed
---
