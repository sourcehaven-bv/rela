rela.bypass_acl(function(admin)
  for _, e in ipairs(admin.list_entities("ticket")) do
    print("- " .. e.id)
  end
end)
