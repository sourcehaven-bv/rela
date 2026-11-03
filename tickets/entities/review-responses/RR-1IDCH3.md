---
id: RR-1IDCH3
type: review-response
title: MessageTypeView.can_post is true for signal types on stdio, which then refuses them
finding: With RefuseSignals, read_channel still advertises ephemeral types as postable, and the agent learns otherwise only through signals_not_kept. Set CanPost false (or add log_only) for ephemeral types when the read comes from a LogOnly transport.
severity: minor
resolution: can_post is false for ephemeral types when the transport is log-only.
status: addressed
---
