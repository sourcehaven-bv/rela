---
id: RR-JZ99BB
type: review-response
title: Create-channel error handling ignores the schema outcome
finding: 'submitCreate (ChannelsSection.vue:115-122) answers every outcome other than ''none'' with list.refetch. For config_changed or channel_type_conflict (classified ''schema'') the descriptor is not reloaded, so the section can keep offering a type the server no longer has. Fix: mirror OpenChannel''s handle(): on ''schema'' call channelSchema.reload() and schema.reloadChannelPresentations(); add create rows to the AC 14 table test.'
severity: minor
resolution: submitCreate reloads the descriptor and the channel presentations on a schema outcome, and the AC 14 tests gained create rows for form, toast and schema outcomes.
status: addressed
---
