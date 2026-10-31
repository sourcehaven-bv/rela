---
id: RR-ZL0J0H
type: review-response
title: Unquoted no and off are accepted as false
finding: 'The key checker accepts whatever yaml.v3 decodes as a bool (keys.go:198-208), so unread: no, unread: off and unread: n are accepted as false. Only the quoted "no" is refused. The negative test table should say the string is quoted, and it can add an unquoted no row that loads as false, so nobody "fixes" it later by accident.'
severity: nit
resolution: ADDRESSED Negative table says the refused "no" is quoted; a TestParse_Unread row pins unquoted no as false.
status: addressed
---
