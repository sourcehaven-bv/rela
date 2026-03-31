---
id: RR-H6RI
type: review-response
title: MetamodelAccessor interface location
finding: MetamodelAccessor interface is defined in workspace but implemented by metamodel.Metamodel. Violates dependency inversion.
severity: significant
reason: Interface is placed near its usage in workspace alongside ValidationFilter. Moving to cli would create circular dependency. Follows Go idiom of interfaces defined near usage.
status: wont-fix
---
