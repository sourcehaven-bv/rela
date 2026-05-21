(entity.status == 'ready' and has_role(current_user, 'triage')) or has_role(current_user, 'admin')
