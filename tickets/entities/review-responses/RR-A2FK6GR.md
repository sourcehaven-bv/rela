---
id: RR-A2FK6GR
type: review-response
title: Request-path scripts read operator secrets.yaml and ai.yaml in tenant mode
finding: The data-entry App builds its own Lua deps for actions, document renders and webhook scripts. Those deps fell back to reading the shared .rela directory, so a tenant's request-path script could read the operator's secrets.yaml and ai.yaml despite the withheld host config.
severity: significant
resolution: 'Fixed in 3a5b2d9ff: dataentry.SetLuaHost (called from dataentrywire.Services) gives the App the host config and seams of its services (appbuild.LuaHost). In tenant mode script.RequireHostConfig makes script.NewWriterRuntime refuse deps with a nil Host instead of reading .rela. Pinned by dataentrywire/withheld_test.go and script/runtime_test.go.'
status: addressed
---
