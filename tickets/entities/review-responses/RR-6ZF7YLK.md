---
id: RR-6ZF7YLK
type: review-response
title: Test paths were read relative to the project root only
finding: A path given from a subdirectory of the project was not found.
severity: minor
resolution: resolveTestPaths reads an existing path from the working directory and makes it root-relative. TestResolveTestPaths covers it.
status: addressed
---
