---
id: RR-6B17EW7
type: review-response
title: File names with quotes or spaces break the directory list
finding: xargs quote handling and for-loop word splitting break odd paths, and core.quotePath escapes non-ASCII paths.
severity: minor
resolution: Directories are read with while IFS= read -r and the diff runs with core.quotePath=false.
status: addressed
---
