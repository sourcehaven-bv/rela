import { describe, it, expect } from 'vitest'
import { resolveUserToken } from './userToken'

describe('resolveUserToken', () => {
  it.each([
    ['resolves $user to the signed-in user', '$user', 'alice', 'alice'],
    ['leaves $user unresolved without a user', '$user', undefined, undefined],
    ['leaves $user unresolved for an unidentified caller', '$user', 'unknown', undefined],
    ['passes other strings through', 'open', 'alice', 'open'],
    ['is case-sensitive', '$USER', 'alice', '$USER'],
    ['passes non-strings through', 3, 'alice', 3],
  ])('%s', (_name, value, user, want) => {
    expect(resolveUserToken(value, user)).toBe(want)
  })
})
