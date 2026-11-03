---
id: RR-NCLPZA
type: review-response
title: Unset axis values and unbounded integer axes
finding: Plan did not say where rows with an unset matrix axis go, or bound an integer axis with many distinct values.
severity: significant
resolution: 'Rows with an unset axis value are counted in an unplaced total and left out of cells (not partial). An integer axis shows the values present in the rows, ascending, capped at 50 with truncated: true. The declared min..max range was dropped: present values are enough for a heat map and need no second code path.'
status: addressed
---
