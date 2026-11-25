---
id: RR-GIHCDNY
type: review-response
title: Text box lacks its own label and live error region
finding: Text box lacks its own label and live error region
severity: minor
resolution: The box has its own aria-label and id; the message sits in a role=status element tied by aria-describedby. The widget's invalid/describedBy stay on the picker.
status: addressed
---
