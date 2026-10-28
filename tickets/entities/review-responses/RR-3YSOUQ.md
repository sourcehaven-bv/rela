---
id: RR-3YSOUQ
type: review-response
title: Editing the docs example ripples into testdata/valid.yaml and its consumers
finding: 'AC 10 and the files list say docs_example_test.go "needs no edit but runs the edited example". TestDocsExample_Realtime (internal/realtimecfg/docs_example_test.go) reads docs-project/entities/guides/GUIDE-realtime.md, not docs/realtime.md. It also requires the guide''s session block to appear verbatim in testdata/valid.yaml. Adding dismiss: in: [detected] to the guide therefore forces the same edit in valid.yaml. That file is loaded by load_test.go (which patches it with exact-line string substitutions, for example session_test.go:248 patches the dismiss line), internal/realtime/reminders_test.go, timers_test.go and internal/appbuild/realtime_handoff_test.go. Any of those that posts dismiss outside detected changes behaviour. The statement in AC 1 that existing suites "pass unchanged" is then not guaranteed. The chosen example also contradicts Q1, whose argument against option (c) is that a lead could no longer post dismiss as a plain message in contained. List valid.yaml and its consumers in the files to modify. Keep docs/realtime.md and the guide in sync. Pick an example type that changes no existing test, or add a new type such as a closing remark with in: [notified].'
severity: significant
resolution: The breach example, the guide block and `testdata/valid.yaml` stay unchanged; `in:` gets its own docs snippet, compiled verbatim by a new `message_in_test.go` case; `valid.yaml` and its consumers are listed as unchanged on purpose, and `docs/realtime.md` and the guide stay in sync.
status: addressed
---
