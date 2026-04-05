---
id: RR-Z9TR
type: review-response
title: RunFile did not preserve filename in chunk name
finding: LoadString does not accept a chunk name parameter. Error messages showed '<string>' instead of the filename. The code comment was misleading.
severity: significant
resolution: Changed to use L.Load(strings.NewReader(code), path) which accepts the filename as chunk name.
status: addressed
---
