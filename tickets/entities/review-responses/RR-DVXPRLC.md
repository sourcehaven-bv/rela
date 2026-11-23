---
id: RR-DVXPRLC
type: review-response
title: Windowed loops hide whole-type scans
finding: Uncapped growing windows or fixed windows to the end of the type drop off the guard while still reading the whole type when matches are sparse.
severity: significant
resolution: 'LLM: no condition or a lowered one reads one bounded page; a non-lowering condition keeps an explicit request-path scan entry. firstNByTypes keeps its three windows and reads the tail through visibleEntitiesOfType, which carries the request-path entry.'
status: addressed
---
