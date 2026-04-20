---
id: RR-P370R
type: review-response
title: Self-echo hashing breaks across decorator boundary
finding: writeFileSealed (markdown.go:496-514) records hashContent(sealed) after sealing. Watcher (watcher.go:179-192) reads raw and hashes raw — contract is hash-of-sealed-bytes. After refactor, fsstore calls bytes.WriteFile(plaintext) and never sees sealed bytes; sealing happens inside EncryptedFS.WriteFile. Recorded hash becomes hash-of-plaintext, but watcher still hashes sealed bytes. They never match — every self-write becomes a self-echo storm followed by Unseal-and-reparse. Plan gestures at this in Hazards but does not commit to a mechanism.
severity: critical
resolution: Plan updated to move hash-on-write into a PostWrite(path, bytes) callback on the lowest-level writer (SafeFS), not EncryptedFS. SafeFS fires the hook exactly once per successful atomic rename with the bytes that landed on disk. fsstore subscribes and calls recordHash(path, bytes). The contract is 'hash what durably sits on disk' — agnostic to what transforms (encryption today, compression tomorrow) sit above. EncryptedFS stays ignorant of self-echo. See Part 1 > Write-hook design in the plan.
status: addressed
---
