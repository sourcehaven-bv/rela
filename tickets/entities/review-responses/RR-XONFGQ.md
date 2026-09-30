---
id: RR-XONFGQ
type: review-response
title: Busy HTTP port sent scenarios to another process
finding: The server logs 'starting server' before binding; kill then failed under set -e.
severity: minor
resolution: run.sh refuses a port that already answers and checks the server is alive before stopping it.
status: addressed
---
