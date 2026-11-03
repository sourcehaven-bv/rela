---
id: RR-CZCL2G
type: review-response
title: Dead ErrConflict branch
finding: ErrConflict with created != nil can only come from the relation insert with a fresh id, and ErrRelationAlreadyExists is not mapped by the handler.
severity: minor
resolution: The dead branch was removed with the converter rewrite.
status: addressed
---
