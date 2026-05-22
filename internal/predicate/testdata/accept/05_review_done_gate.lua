entity.status == 'review'
  and entity.assignee ~= entity.created_by
  and entity.effort ~= nil
