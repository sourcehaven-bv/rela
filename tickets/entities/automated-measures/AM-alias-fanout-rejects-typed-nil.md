---
id: AM-alias-fanout-rejects-typed-nil
type: automated-measure
title: 'Regression test: a disabled subsystem is dropped from the alias fanout'
description: Feeds the real buildComments output (comments disabled, so a nil *comments.Service) into newAliasFanout and asserts the subscriber is dropped and an entity delete does not panic. Deliberately uses the builder's return value rather than a literal nil, because a literal nil becomes a true nil interface and cannot reproduce the typed-nil boxing that produced BUG-N7QF2M. Catches any future "optional subsystem returns a nil *T" wired into an interface parameter. P4 invariant test.
kind: test
location: internal/appbuild/comments_test.go (TestAliasFanout_NilWhenOnlyTypedNilsSubscribe, TestAliasFanout_SkipsTypedNilBesideLiveSubscriber, TestAliasFanout_DisabledCommentsSurviveDelete)
status: active
---
