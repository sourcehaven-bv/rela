// Guard for BUG-762I34: the unit suite must never reach the network.
//
// Before the fix, src/test/setup.ts stubbed localStorage, crypto,
// ResizeObserver and EventSource but not HTTP, so the shared axios instance
// used its real node adapter and dialled localhost:3000. Any component with an
// onMounted fetch (ExportMenu -> getTransforms, reached transitively by
// DynamicForm and the entity/list views) therefore fired a real request. Local
// runs got a fast ECONNREFUSED that the component's own catch swallowed; a
// slower CI runner got `socket hang up` AFTER the triggering test had finished,
// which vitest reports as an unhandled rejection and fails the run on — with
// every test passing.
//
// These assert the property rather than the symptom, because the symptom is
// timing-dependent and does not reproduce locally. If someone removes the
// adapter stub, or adds an api module that builds its own axios instance, this
// fails deterministically here instead of intermittently in someone else's PR.
import { describe, it, expect } from 'vitest'
import axios from 'axios'

import { api } from '@/api/client'
import { getTransforms } from '@/api/transforms'

describe('unit suite never reaches the network', () => {
  it('has a non-default axios adapter installed', () => {
    // The real node adapter is a function named "httpAdapter"; the stub is our
    // own arrow function. Asserting "not the built-in" rather than a specific
    // identity keeps this from breaking if the stub is refactored.
    expect(axios.defaults.adapter).toBeTypeOf('function')
    expect((axios.defaults.adapter as { name?: string })?.name).not.toBe('httpAdapter')
  })

  it('resolves an unmocked GET instead of attempting a connection', async () => {
    // Before the fix this rejected with AggregateError/ECONNREFUSED ::1:3000.
    await expect(api.get('/_probe_no_such_endpoint')).resolves.toBeDefined()
  })

  it('resolves the real onMounted call that caused the flake', async () => {
    // getTransforms() is what ExportMenu calls; it also memoises the promise at
    // module scope, which is why a rejection could outlive the component that
    // created it and end up unowned.
    await expect(getTransforms()).resolves.toBeDefined()
  })
})
