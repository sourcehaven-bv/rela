-- Schedules the selected to-do for today, in the part of the day the action
-- names (rela.params.part_of_day). The date is UTC, as today() is in the
-- list conditions.
if entity == nil then
  return { message = "Select a to-do first", message_type = "warning" }
end
rela.update_entity(entity.id, {
  when = "scheduled",
  start_date = rela.now:sub(1, 10),
  part_of_day = rela.params.part_of_day,
})
