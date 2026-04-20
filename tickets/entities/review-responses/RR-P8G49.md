---
id: RR-P8G49
type: review-response
title: --pub-file has no size guard
finding: internal/cli/keys.go:79 calls os.ReadFile(path) with no length cap. A user who accidentally points --pub-file at a large binary gets that entire file loaded into memory before ParseRecipient rejects it. Not a real attack vector — user picks the file themselves — but well-behaved CLI bounds inputs. Hybrid keys are ~2 KB; 64 KB is generous ceiling.
severity: minor
reason: Low-value input validation. User provides the path themselves — not an attack vector. Malformed content is already rejected by ParseRecipient within milliseconds. Adding a 64 KB size gate would prevent a theoretical misfire but add no real defense. Re-open if this ever becomes a real user-facing issue.
status: wont-fix
---
