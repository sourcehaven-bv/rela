---
id: RR-NEE4FC
type: review-response
title: 'appEntityWriter is not minimal: ValidateCreate is present only to feed a child handler'
finding: App holds a method it never calls because it distributes one narrowed value to sub-handlers
severity: significant
resolution: Adopted the suggested factoring. NewApp now takes the concrete *entitymanager.Manager and passes THAT to the attachmentHandler and writeHandler constructors; so each narrows at its own field. appEntityWriter is now exactly 7 methods - verified equal to App's 7 direct calls; no slack. Also updated the entitymanager package doc's 'one to eight' to 'one to seven' and documented the distributor rule there.
status: addressed
---

App directly calls 7 methods; `ValidateCreate` is on `appEntityWriter` purely
because `writeHandler.entityMutator` needs it. So App's interface is the *union*
of its own needs and its children's, which reintroduces "wide because it is a
distributor" one level down — App holds a method it never invokes, with the
reason buried in prose.

The clean factoring is to stop distributing one narrowed value: have `NewApp`
take the concrete `*entitymanager.Manager` (it is a composition root, it already
takes 13 positional collaborators, and appbuild hands it the concrete type
anyway), store it as a genuinely-minimal 7-method `appEntityWriter`, and pass
the concrete handle to the sub-handler constructors, each narrowing at its own
field.

Consequence worth noting: the `entitymanager` package doc's "between one and
eight of the nine methods" becomes "one to seven".
