---
id: RR-LJ8IB
type: review-response
title: Rewriter decision cases not tabulated
finding: 'Plan says ''strip pre-existing return_to and inject ours'' for form routes but ''always inject on any internal path'' without saying how the form/non-form branches combine with returnPath=set/empty. Write a 2x2 decision table in the plan: (path is form | path is non-form) × (returnPath is set | returnPath is ''''). Each cell specifies: strip pre-existing return_to? inject new one? emit anchor id? Today the plan prose doesn''t unambiguously answer all four.'
severity: significant
resolution: '2x4 decision table added to the Approach section: four path classes (form | non-form internal | external/mailto/anchor | legacy edit:// / create://) crossed with returnPath empty vs set. Each cell specifies the three choices: strip pre-existing return_to, inject new one, emit anchor id.'
status: addressed
---
