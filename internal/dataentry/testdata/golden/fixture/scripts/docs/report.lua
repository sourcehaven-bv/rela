for _, e in ipairs(rela.list_entities("ticket")) do
  print("- " .. e.id .. " " .. tostring(e.properties.title))
end
