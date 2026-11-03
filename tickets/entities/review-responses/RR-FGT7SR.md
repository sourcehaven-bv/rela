---
id: RR-FGT7SR
type: review-response
title: Put all type-button checks in one checker beside choiceChecker
finding: 'The plan splits the type-level checks over validate.go (no session, ephemeral) and session.go (typeButtons after states). choiceChecker already shows the pattern for a message-type key that needs the compiled session: it runs once at validate.go:215, after the session is compiled at validate.go:205-211, and does its ephemeral refusal there (choices.go:28-31). Do the same: one typeButtonChecker{v}.check(raw, ct) called beside choiceChecker, covering every AC 3 case, so the rules for one key live in one function and the no-session case does not need a second hook in messageInCheck.withoutSession.'
severity: minor
resolution: All decision_button checks, the option parsing and the merge live in typeButtonChecker.check, called once beside choiceChecker in validate.go. No hook in messageInCheck.withoutSession.
status: addressed
---
