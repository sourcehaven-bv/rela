---
id: RR-LPTOX
type: review-response
title: No test asserts factory's single-branch guarantee (AC#9)
finding: 'Plan AC#9 said ''test asserts no code path constructs EncryptedFS without wantSealed=true and vice versa.'' Test doesn''t exist. Argument is purely code-reading. Future refactor could split the branch, or caller could pass Bytes: cryptofs.New(...) with WantSealed: false (nothing rejects it). fsstore.New does not cross-validate (Bytes encrypts) == WantSealed.'
severity: significant
resolution: 'Added TestFSFactory_SingleBranchInvariant with two subtests: encrypted branch must produce sealed bytes on disk; cleartext branch must produce plaintext. Both branches use the exact same FSFactory.OpenStore path, proving the single if cfgExists decision controls both decorator install AND wantSealed. Covers AC#9.'
status: addressed
---
