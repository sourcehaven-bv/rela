---
id: RR-XB01RM
type: review-response
title: expect token is an oracle on hidden fields
finding: checkExpect compares the caller token with VersionOf(raw live row), so a caller with partial field visibility can confirm guessed hidden values by tag success vs conflict.
severity: significant
resolution: TagCurrent compares expect with VersionOf of the caller's gated read and passes the raw token to the store; a mismatch is a conflict. Tests TestVersionTags_TokenIsPerReader, _TokenIsNoHiddenFieldOracle, _WriteAfterTokenConflicts.
status: addressed
---
