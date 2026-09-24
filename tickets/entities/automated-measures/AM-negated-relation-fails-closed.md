---
id: AM-negated-relation-fails-closed
type: automated-measure
title: An unanswered relation question is an error, never "no match"
description: relresolve refuses a row it did not answer and an entity with no id (TestAnswersFor_RefusesUnaskedRow, TestBinder_EmptyIDIsAnError), and every surface pins that a traversal error denies, blocks or does not fire even under `not related(...)`. A false answer to an unanswered question passes a negated condition and widens access.
kind: test
location: internal/relresolve/
status: active
---
