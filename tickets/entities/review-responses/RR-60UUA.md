---
id: RR-60UUA
type: review-response
title: matchFrontendRoute as package-level mutable var
finding: 'internal/dataentry/document.go:36 declares matchFrontendRoute as `var ... routeMatcherFunc = func(...) {...}`. Nothing reassigns it, so the var adds no value — just mutable package state a test could accidentally trample. Convert to a plain function and adapt at the call site: routeMatcherFunc(matchFrontendRoute).'
severity: minor
resolution: matchFrontendRoute converted from `var ... routeMatcherFunc = func(...)` to a plain function `func matchFrontendRoute(...)`. Call sites in api_v1.go now wrap it with routeMatcherFunc(matchFrontendRoute). No mutable package state.
status: addressed
---
