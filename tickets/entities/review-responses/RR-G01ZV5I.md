---
id: RR-G01ZV5I
type: review-response
title: Short IDs and automation math.random are not seedable so identical-results AC is untestable
finding: entity/id.go uses crypto/rand; entitymanager/core.go rand.IntN; gopher-lua math.random is global.
severity: significant
resolution: 'AC12 narrowed: t.fake/t.random values and the clock are reproducible from the seed; entity IDs and randomness inside automations are not. Tests refer to IDs through returned values. Documented.'
status: addressed
---
