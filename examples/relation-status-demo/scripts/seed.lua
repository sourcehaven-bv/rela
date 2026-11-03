local function st(titel, volgorde, categorie)
  return rela.create_entity("status", {titel = titel, volgorde = volgorde, categorie = categorie}, "").id
end
local S = {
  backlog = st("Backlog", 10, "open"),
  postpone = st("Postpone", 20, "wachten"),
  bezig = st("In progress", 30, "actief"),
  review = st("Review", 40, "actief"),
  qa = st("QA", 50, "actief"),
  gereed = st("Gereed", 60, "gereed"),
}
local function init(titel, kolommen)
  local id = rela.create_entity("initiatief", {titel = titel}, "").id
  for _, k in ipairs(kolommen) do rela.create_relation(id, "biedt_status", S[k]) end
  return id
end
-- Twee initiatieven met elk een eigen set en volgorde.
local portaal = init("Klantportaal", {"backlog", "postpone", "bezig", "qa", "gereed"})
local audit = init("ISO-audit", {"backlog", "bezig", "review", "gereed"})
local function taak(parent, titel, status)
  local id = rela.create_entity("taak", {titel = titel}, "").id
  rela.create_relation(parent, "bestaat_uit", id)
  rela.create_relation(id, "heeft_status", S[status])
end
taak(portaal, "Inlogscherm ontwerpen", "bezig")
taak(portaal, "SSO koppelen", "backlog")
taak(portaal, "Meldingenoverzicht", "postpone")
taak(portaal, "Statuspagina testen", "qa")
taak(portaal, "Prototype opleveren", "gereed")
taak(portaal, "Toegankelijkheid reviewen", "review") -- niet aangeboden: komt in Other
taak(audit, "Risicoanalyse bijwerken", "review")
taak(audit, "Beleid publiceren", "gereed")
taak(audit, "Interne audit plannen", "backlog")
rela.output({portaal = portaal, audit = audit})
