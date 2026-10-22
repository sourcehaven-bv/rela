---
id: RR-YHIIT5
type: review-response
title: channelTyping wording is unspecified for three typists and plural counts
finding: 'AC 19 gives examples for one, two and four typists only. For three, "Ann, Bo and Cy" and "Ann, Bo and 1 other" are both possible, and "others" needs a plural form for translation. Plan change: signature channelTyping({ names: string[]; others: number }), with the component passing at most two names, plus a three-typist story row.'
severity: nit
resolution: 'Plan updated: channelTyping takes {names, others} with the component passing at most two names; story rows for one, two, three and five typists with "1 other" and "N others".'
status: addressed
---
