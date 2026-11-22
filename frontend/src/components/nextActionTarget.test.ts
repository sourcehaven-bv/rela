import { describe, it, expect } from 'vitest'
import router from '@/router'
import { leadsHere } from './nextActionTarget'

/** Resolved against the real router, so a renamed route or param fails here. */
describe('leadsHere', () => {
  const here = (path: string) => router.resolve(path)

  it('matches the page a navigate offer leads to, query included', () => {
    const target = router.resolve('/entity/account/ACC-1?run=RUN-1')
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-1'))).toBe(true)
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-1&tab=x'))).toBe(true)
  })

  it('does not match another entity, another run or no run', () => {
    const target = router.resolve('/entity/account/ACC-1?run=RUN-1')
    expect(leadsHere(target, here('/entity/account/ACC-2?run=RUN-1'))).toBe(false)
    expect(leadsHere(target, here('/entity/account/ACC-1?run=RUN-2'))).toBe(false)
    expect(leadsHere(target, here('/entity/account/ACC-1'))).toBe(false)
    expect(leadsHere(target, here('/dashboard'))).toBe(false)
  })
})
