---
id: RR-D4KC3
type: review-response
title: Factory UserState==nil auto-construct is a footgun; tests get silent real-FS writes
finding: 'Plan: when factory''s UserState field is nil, auto-build via NewFS. Two factories opened at different entry points can end up with different services if one caller threads UserState and the other doesn''t. Tests that construct FSFactory{FS: mem, Paths: p} get a real-filesystem UserState silently.'
severity: significant
resolution: 'Make construction explicit: NewFSFactory(fs, paths, us) (*FSFactory, error). Remove public struct-literal construction pattern. UserState must be non-nil. Tests use userstate.NewForTest(t.TempDir()). No magic.'
status: addressed
---
