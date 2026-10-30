---
id: RR-G3ZRY5
type: review-response
title: PoC fold drops the revise feedback requirement and the approve note
finding: 'In the PoC, revise requires feedback and approve carries an optional note (rela-agent-runner-poc/rela-mission/project/realtime.yaml:42-49). The folded plan-decision makes feedback optional, so a composer post with decision=revise and no feedback moves the run to planning with nothing for the runner to act on. Add a type-level when: that requires feedback when decision is revise, and confirm that the Approve preset still passes it, which the AC 3 check does. Keep note. Also state that the fold changes the runner, which keys on the type names approve and revise, so the fold is a follow-up and not part of this PR.'
severity: minor
resolution: 'The PoC fold is now a follow-up and is only described, including the feedback-required when: for revise, the kept note, and the runner change.'
status: addressed
---
