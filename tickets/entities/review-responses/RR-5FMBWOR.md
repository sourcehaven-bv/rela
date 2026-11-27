---
id: RR-5FMBWOR
type: review-response
title: aria-selected never true in typeahead
finding: aria-selected compared the option id with the committed label, so no option was marked selected.
severity: significant
resolution: Compare with modelValue; test pins the selected option.
status: addressed
---
