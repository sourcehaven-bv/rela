import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import RlAttachmentCard from 'rela-components/components/task/RlAttachmentCard.vue'
import { attachmentKind } from 'rela-components/components/task/attachmentKind'

describe('RlAttachmentCard', () => {
  it('is a download link when it has an href', () => {
    const w = mount(RlAttachmentCard, {
      props: { attachment: { id: 'a', name: 'Plan.pdf', kind: 'pdf', href: '/files/a' } },
    })
    const link = w.find('a.rl-attachment-card__main')
    expect(link.attributes('href')).toBe('/files/a')
    expect(link.attributes('download')).toBe('Plan.pdf')
    expect(w.text()).toContain('PDF · Download')
  })

  it('is a button that emits click without an href', async () => {
    const attachment = { id: 'a', name: 'Plan.pdf', kind: 'pdf' as const }
    const w = mount(RlAttachmentCard, { props: { attachment } })
    await w.find('button.rl-attachment-card__main').trigger('click')
    expect(w.emitted('click')).toEqual([[attachment]])
  })

  it('offers remove only when removable, outside the link', async () => {
    const attachment = { id: 'a', name: 'Plan.pdf', kind: 'pdf' as const, href: '/files/a' }
    const plain = mount(RlAttachmentCard, { props: { attachment } })
    expect(plain.find('.rl-attachment-card__remove').exists()).toBe(false)

    const w = mount(RlAttachmentCard, { props: { attachment, removable: true } })
    const remove = w.find('.rl-attachment-card__remove')
    expect(remove.attributes('aria-label')).toBe('Remove Plan.pdf')
    expect(w.find('a .rl-attachment-card__remove').exists()).toBe(false)
    await remove.trigger('click')
    expect(w.emitted('remove')).toEqual([[attachment]])
  })

  it('shows a preview in place of the kind icon', () => {
    const w = mount(RlAttachmentCard, {
      props: { attachment: { id: 'a', name: 'shot.png', kind: 'image', preview: '/thumb' } },
    })
    expect(w.find('img.rl-attachment-card__preview').attributes('src')).toBe('/thumb')
  })
})

describe('attachmentKind', () => {
  it.each([
    ['report.pdf', undefined, 'pdf'],
    ['scan', 'application/pdf', 'pdf'],
    ['photo.bin', 'image/png', 'image'],
    ['Notes.DOCX', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'doc'],
    ['budget.xlsx', undefined, 'sheet'],
    ['archive.zip', 'application/zip', 'other'],
    ['README', undefined, 'other'],
  ])('%s (%s) is %s', (name, type, kind) => {
    expect(attachmentKind(name, type)).toBe(kind)
  })
})
