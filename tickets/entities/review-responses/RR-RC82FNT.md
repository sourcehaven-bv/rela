---
id: RR-RC82FNT
type: review-response
title: Malformed peer element kind panics
finding: The Element doc invited filling fields from peer input; Fingerprint panics on any kind byte other than E or R so a peer could crash the process.
severity: significant
resolution: Added ParseKind(byte) (Kind and error). The Element and Fingerprint docs require decoders to use it so only a programming error reaches the panic. Tested.
status: addressed
---
