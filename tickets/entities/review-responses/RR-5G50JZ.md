---
id: RR-5G50JZ
type: review-response
title: projectPath no longer cleans the script path
finding: 'dir + ''/'' + name kept ''./'' segments, so script: ./stamp.lua showed as scripts/./stamp.lua.'
severity: nit
resolution: projectPath uses filepath.Join then ToSlash.
status: addressed
---
