---
id: RR-L1UMYS
type: review-response
title: The fold and its last message share one data-seq
finding: 'Plan step 4 puts data-seq = last seq on the fold row. When the run is expanded, the last message also renders data-seq (RlChannelMessage.vue:195). firstVisible (RlChannel.vue:225), jumpTo (RlChannel.vue:308) and focusRow (RlChannel.vue:320) all take the first match in DOM order, which is the fold button. So a reply quote or a focusRow on that message focuses the fold button instead. Change the plan: give the fold its own attribute (for example data-fold-seq), add it to the firstVisible selector, and keep data-seq only on message rows.'
severity: significant
resolution: The fold li now carries data-fold-seq and never data-seq, so jumpTo and focusRow find only message rows, and firstVisible selects both attributes (steps 4 and 6). Added the AC 6 case for a reply quote to the last message of an expanded run.
status: addressed
---
