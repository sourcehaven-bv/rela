---
id: AM-default-world-generated-only-without-worlds
type: automated-measure
title: The default world exists only when no worlds are declared
description: With declared worlds, ?world=default is not a world, and a schema declaring more than one world must set default_world or fail to load (a single declared world is the default); without worlds, the generated default world shows faced entities.
kind: test
location: internal/worlds + internal/dataentry
status: proposed
---
