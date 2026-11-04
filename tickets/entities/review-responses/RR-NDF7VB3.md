---
id: RR-NDF7VB3
type: review-response
title: Missing tests for edge paths
finding: No tests for the edit/restore case, openForm without a session, refused after a move with a button type, Go to decision returning false, arrows with nothing checked.
severity: minor
resolution: 'Added: EditButtonTypeKeepsDecision, NoneCheckedArrows, ''a form refusal after the session moved records nothing and toasts'', ''without a session, Other opens the answer form with nothing recorded'', and the Go to decision in-flight test. The no-session preset branch is unchanged code and already covered by existing preset tests.'
status: addressed
---
