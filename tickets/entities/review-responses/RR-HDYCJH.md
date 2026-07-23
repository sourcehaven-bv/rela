---
id: RR-HDYCJH
type: review-response
title: 'transform arch-lint grants unused mayDependOn: script'
finding: 'The transform component''s arch-lint block allows mayDependOn: script for Lua overrides, but the Lua override lives in dataentry.exportRenderer (RendererFunc around documents.RenderMarkdown); internal/transform imports only cmdexec + metamodel. The script allowance is dead surface that weakens the boundary. Drop it until transform genuinely imports script.'
severity: nit
resolution: 'Dropped mayDependOn: script from the transform arch-lint component; it now allows only cmdexec + metamodel (which is all internal/transform imports). arch-lint still green. The Lua/command override lives in dataentry, as documented in the updated comment.'
status: addressed
---
