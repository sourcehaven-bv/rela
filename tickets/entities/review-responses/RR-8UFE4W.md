---
id: RR-8UFE4W
type: review-response
title: The JSON tag -- the actual SPA contract -- is pinned by nothing
finding: |-
    responses.go carries a doc comment claiming 'the compiler is what keeps the wire surface and the internal DTO from drifting.' That is only two-thirds true.

    The reviewer tested it four ways. Deleting or reordering a field IS caught -- Go's struct-conversion rule requires identical field names in identical order, which is stronger than the comment implies. But renaming the tag `json:"span"` to `json:"columnSpan"` compiles clean and passes the ENTIRE test suite (dataentry and apiwire/v1 both ok).

    The compiler pins Go-side shape. Nothing pins the JSON key the frontend actually reads. Neither sections_span_test.go nor any apiwire test ever marshals a v1.SectionField, so a one-character tag typo would ship a silently span-less UI with green tests.

    Fix is one golden assertion: marshal a v1.SectionField with Span set and assert the serialized key is `span`.
severity: significant
resolution: |-
    Fixed in 2ff8e0db. Added internal/apiwire/v1/section_field_test.go with two assertions: TestSectionFieldWireKeys marshals a fully-populated SectionField and checks every JSON key the SPA reads (including `"span":4`), and TestSectionFieldOmitsEmptySpan pins that an unauthored span stays OFF the wire rather than serializing as `"span":0` -- the full-width default belongs to the CSS fallback alone, and emitting 0 would put a second, conflicting default on the wire plus a key on every field of every response.

    Also corrected the doc comment on the struct. The old wording claimed the compiler keeps the wire surface from drifting; the review showed that is true for SHAPE (deletion and reordering both fail the build, which is stronger than I had assumed) but not for tags. The comment now says which half the compiler covers and why the tag needs its own test.
status: addressed
---
