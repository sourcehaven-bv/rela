---
id: RR-IZNXID
type: review-response
title: Poll replays every channel target on any wake and takes the store-wide channels flock per Read
finding: Section 3.3 ignores Take() keys, so any post in the process replays all read_channel targets. Each Read does Head under the filechannels store-wide lock (ops.go Head), up to 512 targets per poll period on remote, each digesting up to about 800 KB. Filter wakes by the target's key and lower the watch replay limit; neither leaks, because a hidden replay digest is constant.
severity: minor
resolution: Wakes are filtered by the target's channel key. Channel targets use a lower replay page limit.
status: addressed
---
