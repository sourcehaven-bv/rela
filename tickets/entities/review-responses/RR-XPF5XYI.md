---
id: RR-XPF5XYI
type: review-response
title: Rows without a date use up max_rows under desc
finding: The store sorts missing values first when descending. Under order desc rows with no date load first and push out the newest rows.
severity: significant
resolution: Documented in the guide with the advice to filter on the date. The No date section stays last. Store null order is unchanged (store-wide and pinned by storetest).
status: addressed
---
