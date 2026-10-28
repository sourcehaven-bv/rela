---
id: RR-LWKZC5
type: review-response
title: 'File tier: two full tree walks per inbox call, and edit-add failures that never heal'
finding: 'Each inbox call (and each watch replay) walks every channel directory once for ListReads and again for ListAddressed. Read both files in one walk, for example a package-internal helper that returns markers and rows together, or accept the cost and add it to the fs scale note. Second issue: when the mentions.json write after an edit''s rewrite fails, it is swallowed, but Message.Mentions already names the target. The next edit treats the target as "kept", leaves its row alone, and so never re-creates it. Fix: on every row write for a message, rebuild that message''s rows from Message.Mentions plus the reply rule, so the next write heals it. Name the write-after point explicitly: mutate has no hook after rewrite, so Edit needs its own post-rewrite step inside the lock.'
severity: minor
resolution: Two walks accepted and added to the fs scale note; rows rebuilt from Message.Mentions on every write; own post-write step inside the lock for Append and Edit; TestAddressed_Heals.
status: addressed
---
