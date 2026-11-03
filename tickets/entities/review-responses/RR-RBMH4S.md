---
id: RR-RBMH4S
type: review-response
title: Channel handler tests run against copies of the production adapters
finding: channels_env_test.go copies aclAccessSource, anchorReader and refResolver, so AC2, AC4 and AC14 are proven against the copies; a change to the shipped adapters keeps every dataentry test green.
severity: significant
resolution: The dataentry tests build the channel service through the production constructor via appbuildtest.WithChannels; the copied adapters are deleted.
status: addressed
---
