---
id: RR-0NEKQ9
type: review-response
title: JSON Feed has no item cap
finding: XML feeds are bounded by the element cap but JSON only by bytes; 10 MiB of {} is millions of items and Lua tables.
severity: significant
resolution: feed.MaxItems (10 000) for every format; JSON Feed items are streamed with json.Decoder so the cap applies before allocation. TestParse_ItemCap.
status: addressed
---
