---
id: RR-ZPNOQI
type: review-response
title: Refusing an explicit empty display needs a raw-YAML check
finding: 'With Display string and omitempty (plan step 1), the struct decode cannot tell display: "" (or display: null) from an absent key, so validateChannelsShape cannot refuse it from cfg. Other display keys in data-entry.yaml treat empty as the default (config.go:1499). Change the plan: either accept "" as ''shown in full'', which is consistent and simpler, or say the check runs on the raw map in channelsUnknownKeys (channels.go:136-162). Pick one in AC 1.'
severity: minor
resolution: Chose to accept an empty or null display as shown in full, matching the other display keys, so no raw-YAML check is needed (AC 1, step 1). A display empty case is added and the alternative is recorded as rejected.
status: addressed
---
