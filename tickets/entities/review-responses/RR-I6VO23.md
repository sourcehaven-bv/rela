---
id: RR-I6VO23
type: review-response
title: NewScriptReader type-asserts a Resolver back-channel
finding: NewScriptReader used reader.(resolverSource); a Reader without Resolver compiled and failed only at runtime.
severity: significant
resolution: Resolver() is now a method of visibility.Reader (both implementations already had it); NewScriptReader rejects a nil Resolver. Pinned by TestNewScriptReader_RequiresAResolver.
status: addressed
---
