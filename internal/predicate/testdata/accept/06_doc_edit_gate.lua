entity.frozen_for_audit == false
  and (entity.created_by == current_user.id
       or has_role(current_user, 'docs-editor'))
