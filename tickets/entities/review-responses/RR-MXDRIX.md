---
id: RR-MXDRIX
type: review-response
title: Each JSON Lines record of rela channel watch should carry notice first
finding: Section 4 says -o json watch prints one object per line, but AC6 names only "every content-bearing result". Make each line explicitly a {notice, ...} object.
severity: nit
resolution: Each JSON Lines record is a {notice, ...} object.
status: addressed
---
