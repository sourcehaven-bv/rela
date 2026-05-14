import { describe, it, expect, beforeEach } from 'vitest'
import { registerForm, anyFormDirty, _registrySize, _registryClear } from './dirtyFormRegistry'

describe('dirtyFormRegistry', () => {
  beforeEach(() => {
    _registryClear()
  })

  it('starts empty', () => {
    expect(_registrySize()).toBe(0)
    expect(anyFormDirty('TKT-001', 'title')).toBe(false)
  })

  it('reports dirty after register when callback returns true', () => {
    registerForm('TKT-001', (prop) => prop === 'title')
    expect(anyFormDirty('TKT-001', 'title')).toBe(true)
    expect(anyFormDirty('TKT-001', 'status')).toBe(false)
  })

  it('returns false for unregistered entities', () => {
    registerForm('TKT-001', () => true)
    expect(anyFormDirty('TKT-002', 'title')).toBe(false)
  })

  it('supports two forms on the same entity (RR-Z5PQ2)', () => {
    // Side panel says title is dirty.
    registerForm('TKT-001', (prop) => prop === 'title')
    // Main page says status is dirty.
    registerForm('TKT-001', (prop) => prop === 'status')

    expect(anyFormDirty('TKT-001', 'title')).toBe(true)
    expect(anyFormDirty('TKT-001', 'status')).toBe(true)
    expect(anyFormDirty('TKT-001', 'unrelated')).toBe(false)
  })

  it('unregister removes only the calling form', () => {
    const unregA = registerForm('TKT-001', (prop) => prop === 'title')
    registerForm('TKT-001', (prop) => prop === 'status')

    unregA()

    expect(anyFormDirty('TKT-001', 'title')).toBe(false)
    expect(anyFormDirty('TKT-001', 'status')).toBe(true)
  })

  it('removes the entity entry when last form unregisters', () => {
    const u = registerForm('TKT-001', () => true)
    expect(_registrySize()).toBe(1)
    u()
    expect(_registrySize()).toBe(0)
  })

  it('repeated mount/unmount cycles do not leak (HMR coverage)', () => {
    for (let i = 0; i < 5; i++) {
      const u1 = registerForm('TKT-001', () => false)
      const u2 = registerForm('TKT-001', () => false)
      u1()
      u2()
    }
    expect(_registrySize()).toBe(0)
  })
})
