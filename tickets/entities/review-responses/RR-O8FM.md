---
id: RR-O8FM
type: review-response
title: Test only covers cards display + outgoing traversal
finding: 'TestV1Views_NoAddOrLinkInfoOnSections covers exactly one shape (display: cards, outgoing Follow). The historical resolver at sections.go:351-355 branches on FollowIncoming with a different linkAs, and the previous frontend rendered buttons on cards/list/table alike. A regression that re-introduces addInfo/linkInfo only on the FollowIncoming branch would slip past this test.'
severity: significant
resolution: 'Restructured TestV1Views_NoAddOrLinkInfoOnSections into a table-driven test with 5 sub-tests covering: outgoing+cards+with-form, outgoing+list+with-form, outgoing+table+with-form, incoming+cards+with-form, and outgoing+cards+no-form. Extracted assertViewSectionsLackKeys helper for reuse. All variants pass.'
status: addressed
---
