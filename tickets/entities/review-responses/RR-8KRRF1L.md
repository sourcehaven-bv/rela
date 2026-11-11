---
id: RR-8KRRF1L
type: review-response
title: FIFO swapped in between Lstat and Open can block
finding: A FIFO named remote_credential.* swapped in after Lstat makes os.Open block.
severity: minor
reason: Needs write access to the directory holding the declared file and only blocks the pull; O_NONBLOCK is not portable to Windows. The Lstat check refuses a FIFO that is there before the race.
status: deferred
---
