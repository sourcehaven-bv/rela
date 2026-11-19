import axios from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { deleteAttachment, uploadAttachment } from './attachments'
import { setCurrentAsOf } from './asOf'

describe('attachment writes at a past instant', () => {
  afterEach(() => {
    setCurrentAsOf(undefined)
    vi.restoreAllMocks()
  })

  it('refuses an upload without sending it', async () => {
    const put = vi.spyOn(axios, 'put')
    setCurrentAsOf('2025-12-31T23:59:59Z')
    const file = new File(['x'], 'a.txt')
    await expect(uploadAttachment('ticket', 'TKT-1', 'files', file)).rejects.toMatchObject({
      status: 422,
    })
    expect(put).not.toHaveBeenCalled()
  })

  it('refuses a delete without sending it', async () => {
    const del = vi.spyOn(axios, 'delete')
    setCurrentAsOf('2025-12-31T23:59:59Z')
    await expect(
      deleteAttachment('/api/v1/tickets/TKT-1/_attachments/files/a.txt')
    ).rejects.toMatchObject({
      status: 422,
    })
    expect(del).not.toHaveBeenCalled()
  })
})
