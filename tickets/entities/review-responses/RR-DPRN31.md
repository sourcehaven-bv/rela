---
id: RR-DPRN31
type: review-response
title: Corpus copies production wiring by hand
finding: wireGoldenServices copies dataentrywire.Services and the NewApp call copies buildApp; production drift would pass.
severity: significant
resolution: Corpus moved to package dataentry_test and wired through dataentrywire.Services; buildApp lives in package main, so its call order is repeated on appbuildtest services (NewFromCollaborators now builds piles and CalDAV aliases as assembly does).
status: addressed
---
