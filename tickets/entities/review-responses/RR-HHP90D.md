---
id: RR-HHP90D
type: review-response
title: Two tests asserted properties that could not fail
finding: '(a) TestUnrestricted_NilStoreYieldsNil compared a *UnrestrictedReader against nil — a pointer comparison that passed trivially. Its own comment claimed to pin ''a nil store must not produce a reader that panics on first use'' and ''the Lua binding DENIES instead'', and it verified neither: it never assigned to an interface and never called a method. That green check is why the false nil claim survived. (b) appbuild_test.go TestDiscover_LuaDepsDerivable asserts read.VisibleReader != nil, which can no longer fail for VisibleReader now that Unrestricted always returns a non-nil value — it degraded from ''the reader is wired'' to ''Unrestricted returns something''.'
severity: significant
resolution: (a) Replaced with TestUnrestricted_NilStorePanics (recover-based, mutation-verified against a revert to `return nil`) plus TestUnrestricted_SatisfiesReadSurfaceNonNil, which assigns through a locally-declared structural interface (visibility cannot import lua) and checks reflect.ValueOf(r).IsNil() — the linter's nilness check independently confirmed a direct `== nil` there is statically impossible, which is the same defect in miniature. (b) Added a behavioral assertion that the reader actually reaches a store (a missing entity must surface an error), with a comment explaining why the nil check alone is no longer sufficient.
status: addressed
---
