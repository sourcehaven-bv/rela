---
id: RR-S1RROIE
type: review-response
title: ErrPropsNotPushable name no longer matches its use
finding: The error now also covers Related on a reader without pushdown, but its name and message speak of props.
severity: minor
resolution: The message and doc now describe any page filter that cannot be pushed. The identifier is kept to avoid churn in callers.
status: addressed
---
