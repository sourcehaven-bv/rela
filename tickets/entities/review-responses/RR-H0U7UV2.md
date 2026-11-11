---
id: RR-H0U7UV2
type: review-response
title: 'Design: representation of a list-valued to:'
finding: Branch.To, Decide's Target, BranchCheck.To, flowview.Branch.To, check.go and mermaid all assume one target.
severity: significant
resolution: 'Plan updated: Branch keeps To for an ordinary branch and gains Fork []string (To empty) plus Targets() for graph walks; the engine handles the fork after Decide; the wire Branch gains fork.'
status: addressed
---
