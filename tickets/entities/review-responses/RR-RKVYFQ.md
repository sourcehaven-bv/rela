---
id: RR-RKVYFQ
type: review-response
title: A --run-as flag that equals RELA_RUN_AS is ignored without a message
finding: 'runKong sets runAsFromFlag to cli.RunAs != os.Getenv(RELA_RUN_AS). When an operator exports RELA_RUN_AS=system:ci and also passes --run-as system:ci to a command that acts as no one, the value is treated as coming from the environment and is dropped, not refused. The resolution says an explicit flag is refused. Fix: detect the flag from kong''s parse trace (for example, check ktx.Flags for a value set by the flag rather than by the env default), or document the exception.'
severity: nit
resolution: runKong now checks kong's parse path (flagGiven) instead of comparing with RELA_RUN_AS, so an explicit --run-as is refused on a command that acts as no one even when it equals the env value. TestFlagGiven covers both forms with the env set; documented.
status: addressed
---
