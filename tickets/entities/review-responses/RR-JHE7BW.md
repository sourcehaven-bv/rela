---
id: RR-JHE7BW
type: review-response
title: Fixed docker container name collided across checkouts
finding: A second run's stop_pg removed the first run's container.
severity: minor
resolution: Container name carries a checksum of the checkout path.
status: addressed
---
