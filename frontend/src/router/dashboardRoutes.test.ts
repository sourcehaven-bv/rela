import { describe, it, expect } from 'vitest'
import router from './index'

/**
 * Named dashboards (TKT-5LQNLY) live at `/dashboard/<name>`, beside the
 * default `/dashboard`, and answer under a space prefix like every page.
 */
describe('dashboard routes', () => {
  it('resolves the default dashboard without a name', () => {
    const r = router.resolve('/dashboard')
    expect(r.name).toBe('dashboard')
    expect(r.params.name).toBeUndefined()
  })

  it('resolves a named dashboard and passes the name as a prop', () => {
    const r = router.resolve('/dashboard/crm')
    expect(r.name).toBe('named-dashboard')
    expect(r.params).toEqual({ name: 'crm' })
    expect(r.matched[r.matched.length - 1].props.default).toBe(true)
  })

  it('resolves a named dashboard under a space prefix', () => {
    const r = router.resolve('/s/crm/dashboard/pipeline')
    expect(r.name).toBe('named-dashboard')
    expect(r.params).toEqual({ space: 'crm', name: 'pipeline' })
  })
})
