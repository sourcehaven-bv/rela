-- Marks the selected project reviewed today.
if entity == nil or entity.type ~= "project" then
  return { message = "Select a project first", message_type = "warning" }
end
rela.update_entity(entity.id, { last_reviewed = rela.now:sub(1, 10) })
return { message = "Reviewed " .. entity.properties.title, message_type = "success" }
