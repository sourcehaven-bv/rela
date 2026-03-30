---
finding: 'Plan doesn''t address whether builder instances are reusable. If a test does `b := Entity("ticket"); e1 := b.With("x", 1).Build(); e2 := b.With("y", 2).Build()`, will e2 have both x and y? This could lead to subtle test bugs. Document expected behavior: either builders are single-use (Build() clears state or panics on reuse) or explicitly shared state.'
id: RR-7rum
resolution: 'Builders are single-use by convention in Go. Each fluent method returns a new builder or mutates and returns self. Will document: builders should not be reused after Build() - each call to Entity()/EntityFor() creates a fresh builder.'
severity: minor
status: addressed
title: Missing test for reusability of builder instance
type: review-response
---
