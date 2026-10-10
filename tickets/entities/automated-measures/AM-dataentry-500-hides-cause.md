---
id: AM-dataentry-500-hides-cause
type: automated-measure
title: 'Test: dataentry 500s carry only literal details and log the cause'
description: 'Guards against BUG-Y34ZSZ (GitHub #1774). TestNoInternalErrorDetail parses the package and fails on any call or composite literal naming http.StatusInternalServerError whose other arguments are not literals, http constants or w/r/correlationID, so a detail built from an error under any name cannot reach a 500. TestWriteInternalError_HidesTheCause pins that the helpers hide the cause from the client, log it, and stay silent on context.Canceled.'
kind: test
location: internal/dataentry/internalerror_test.go:TestNoInternalErrorDetail, TestWriteInternalError_HidesTheCause, TestCloneEntity_UniqueCollisionIs422
status: active
---
