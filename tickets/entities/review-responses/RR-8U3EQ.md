---
id: RR-8U3EQ
type: review-response
title: Explicit IDType in dataentry test fixture
finding: 'Adding IDPrefix without IDType leaves types as implicit short, silently depending on the default. Set IDType: metamodel.IDTypeShort explicitly.'
severity: nit
reason: The default is the documented, stable behavior of the metamodel. Explicitly setting it in every test fixture adds noise. If the default ever changes (unlikely, given backward-compatibility constraints), many existing projects and tests would need updating in a coordinated way -- not just this fixture.
status: wont-fix
---
