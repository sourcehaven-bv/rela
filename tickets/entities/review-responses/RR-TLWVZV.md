---
id: RR-TLWVZV
type: review-response
title: 'Sibling surfaces did not parse the address: _sidepanel, _history, commands, export, _position'
finding: The address grammar was applied to GET/PATCH/DELETE/_views but not to _sidepanel (gated and loaded the raw string), _history (a faced id yielded a plausible empty timeline at 200), the command runner's entity_id (raw store GetEntity, backend-divergent), export (fsstore rendered, pgstore 404'd) and the SPA's scope navigation (passed the faced route param to _position).
severity: significant
resolution: _sidepanel parses the address (sidePanelEntry), gates the bare id and loads the row; _history parses it and an explicit face names the timeline directly; the command runner parses the state ref and reads GetEntityState; export parses it and serves the bare face (a non-bare address is the uniform not-found, exporting faces is recorded on TKT-5SZG2L); EntityDetail passes the bare id to scope navigation.
status: addressed
---
