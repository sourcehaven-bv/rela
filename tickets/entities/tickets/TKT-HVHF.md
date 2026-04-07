---
id: TKT-HVHF
type: ticket
title: Add server-side actions to data-entry (Lua scripts with redirect/message responses)
kind: enhancement
priority: medium
effort: m
status: done
---

## Description\n\nAdd a new `actions` concept to the data-entry app that lets users trigger Lua scripts from the sidebar. Actions are configured in `data-entry.yaml` and execute server-side. Primary use case: a 'Today's Note' button that finds-or-creates a daily-note for today and redirects to it.\n\n## Config\n\n`yaml\nactions:\n  today_note:\n    description: Open or create today's daily note\n    script: today-note.lua\n\n  find_or_create:\n    script: find-or-create.lua\n    params:\n      entity_type: daily-note\n      key_property: date\n\nnavigation:\n  - label: Today's Note\n    action: today_note\n`\n\n## Script contract\n\n- Scripts live in `actions/` at project root\n- Receive static string params from config via `rela.params`\n- Return a table with optional `redirect`, `message`, `message_type` fields\n- No param interpolation — scripts use `rela.today` etc. for dynamic values\n\n`lua\n-- actions/today-note.lua\nlocal today = rela.today\nlocal notes = rela.list_entities(\"daily-note\", \"date=\" .. today)\n\nlocal note = #notes > 0 and notes[1] or rela.create_entity(\"daily-note\", {\n    date = today,\n    title = today,\n})\n\nreturn {\n    redirect = \"/entity/daily-note/\" .. note.id,\n}\n`\n\n## Backend\n\n- New endpoint `POST /api/v1/_action/{id}`\n- Resolves action from config, runs script with `rela.params` populated\n- Returns script's return table as JSON\n- Errors → 500 + generic toast, full error in server logs\n\n## Frontend\n\n- Nav entry with `action:` renders as a button\n- Click → POST to endpoint, handle `redirect` (navigate) and `message` (toast)"
