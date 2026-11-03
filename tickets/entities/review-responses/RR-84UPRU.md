---
id: RR-84UPRU
type: review-response
title: RlChannelDecisions cannot be reused "as is"
finding: Plan line 267-268 and step 9 say the question row reuses RlChannelDecisions as is, labelled "Choices". The component hard-codes the group label to the "decisions" text (RlChannelDecisions.vue:49-52), the `workflow` icon (line 58) and a hint about state moves (`channelDecisionHint`, line 41-43). Add a `label` prop (and an optional icon) and say what the hidden hint says for a choice; list RlChannelDecisions.vue and its stories under files to modify.
severity: minor
resolution: RlChannelDecisions gains optional label and icon props and a choice hint ("Posts this answer as a reply"); the component and its stories are in the file list.
status: addressed
---
