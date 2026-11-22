import { t } from '@/i18n'

/**
 * The readable name of a principal, by the rules of `actorText` in
 * components/entity/changeText.ts. The flow run timeline uses it too, so a
 * bot reads "System" there as it does in Changes (BUG-UZGMHIK). It lives in
 * utils so the flows components need not import the entity ones.
 */
export function actorLabel(
  principal: string | undefined,
  titles: Record<string, string> = {}
): string {
  const user = principal ?? ''
  if (titles[user]) return titles[user]
  const idpEvent = cut(user, 'webhook:')
  if (idpEvent) return t('audit.changes.actor.idp', { event: idpEvent })
  const hook = cut(user, 'system:webhook:')
  if (hook) return t('audit.changes.actor.webhook', { name: hook })
  const integration = cut(user, 'integration:')
  if (integration) return t('audit.changes.actor.integration', { name: integration })
  if (!user || user === 'unknown' || user.startsWith('system:')) {
    return t('audit.changes.system')
  }
  return user
}

/** The rest of `s` after `prefix`, or '' when it does not start with it. */
function cut(s: string, prefix: string): string {
  return s.startsWith(prefix) ? s.slice(prefix.length) : ''
}
