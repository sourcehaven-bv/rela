---
id: RR-5RDL03
type: review-response
title: 'Config reset replays then: steps on imported entities'
finding: 'After a mapping change the seen-set reset ran ingest for every item; find hit existing entities and then: steps appended and set again on up to max_items entities.'
severity: significant
resolution: 'A reset record carries Rebaseline; until a poll completes, the mapping runs without then: steps. TestRun_RebaselineSkipsSteps; docs explain it.'
status: addressed
---
