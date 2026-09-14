---
id: RR-2KOT6F
type: review-response
title: copydef_unmarshal_test comment promises diagnostic value the ANDed assertion does not deliver
finding: The new `Messages.Notice` check ANDs two independent facts (en and nl) into one bool, so the failure message names the key and prints nothing about which entry was wrong. The pre-existing `Messages` entry has the same shape, so this matches the file's convention — but the added comment makes an explicit claim about mutation-resistance ('a shadow struct that carried read_only and forgot notice fails here') that the assertion's diagnostics do not support. Either split into two keys or trim the comment's promise. The underlying coverage claim does hold, verified via the other test.
severity: minor
resolution: Split the ANDed check into `Messages.Notice[en]` and `Messages.Notice[nl]`, so a failure names which entry dropped the key. Trimmed the comment to match what the assertion now delivers, keeping the point that `en` declares notice as its ONLY message — the case that catches a shadow struct carrying read_only and forgetting notice.
status: addressed
---
