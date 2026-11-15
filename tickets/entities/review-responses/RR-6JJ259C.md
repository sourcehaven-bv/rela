---
id: RR-6JJ259C
type: review-response
title: HighestID parsing and case differ per backend
finding: HighestID parses a leading digit run on fs/mem/pg and strict digits on sqlite; fs/mem/pg compare the prefix case-sensitively, so a mixed-case id is not seen.
severity: minor
resolution: 'Fixed in BUG-RLBXTG5: entity.SequentialNumber is the one matching rule (ASCII case-folded prefix and dash, digits-only suffix) that all four backends call; pgstore reads a range on lower(id) served by the existing lower() indexes; storetest pins case folding and digit parsing on every backend.'
reason: Needs a store contract change with storetest pins on all four backends; ids are minted upper-case, so this only affects hand-written mixed-case ids.
status: addressed
---
