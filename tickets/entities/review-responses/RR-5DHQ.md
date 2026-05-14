---
id: RR-5DHQ
type: review-response
title: e2e top-result test couples to Bleve relevance ranking — will flake on indexer changes
finding: |-
    `e2e/tests/markdown-editor-entity-ref.spec.ts` last test ('typing the target title surfaces it as a top result (RR-Z9C1)') uses a randomized title token (`pickerzzz` + 6 random chars) and asserts `entityPickerOptions.first().toContainText(targetId)`.

    The test's correctness depends on Bleve's TF-IDF scoring putting a unique tokenized term ahead of every seeded fixture. Today that holds because the seeded fixtures have no `pickerzzz*` tokens. But:
      - A future indexer change (analyzer swap, stemming addition, BM25F field weighting tune) can re-order results.
      - A future test fixture that happens to contain `pickerzzz` as a substring of a longer token (test data drift) breaks ranking.
      - The test does not specify WHY this ranks first; it asserts a coincidence of Bleve's default scoring.

    This is the classic 'tests behavior or coincidence' question. The desired UX guarantee is 'exact title match ranks above unrelated entities.' The test as written asserts something narrower: 'this random token, which is the only token in the entire index that matches, lands first.' True but tautological — if there's exactly one matching document, of course it's first.

    Fix: seed two entities, one with title containing `pickerzzz<rand>` and one with title `pickerzzz<rand>2` (or any related term so both match). Assert that the first option text equals the EXACTLY-matching entity's title — a real ranking claim. Alternatively, drop this test: it's not asserting anything the lookup test isn't already covering via 'press Enter inserts the target ID'.

    The other tests in the file are well-structured (round-trip persistence, escape behavior, baseline-vs-after for non-modification). This one is the outlier.
severity: minor
reason: The e2e is a sanity check that the unique title token surfaces -- not a Bleve relevance pinning. The test guards against 'user types my exact title token, picker shows nothing' which is the actual user-facing regression we care about. A future indexer change shuffling ranking would either still match (test passes) or break the search entirely (test fails for a meaningful reason).
status: deferred
---
