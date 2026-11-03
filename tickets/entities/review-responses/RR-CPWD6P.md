---
id: RR-CPWD6P
type: review-response
title: Module compile errors point at the caller
finding: A syntax error in a module is raised from Go via RaiseError, so the frame/line/excerpt point at the require() line in the caller instead of the broken module line.
severity: minor
resolution: Compile errors and errors at a module's top level record a pending frame at the module line, which the runtime's message handler prepends; a caught error does not leak into a later one. Pinned by TestRequire_CompileErrorPointsAtModuleLine, TestRequire_TopLevelErrorFrameNamesModule, TestRequire_CaughtCompileErrorDoesNotLeakIntoLaterError.
status: addressed
---
