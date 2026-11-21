import type { Meta, StoryObj } from '@storybook/vue3-vite'
import RlRenderedDiff from './RlRenderedDiff.vue'

const before = `# Information security policy

This policy applies to all **employees** and contractors of Sourcehaven.

## Scope

The policy covers all systems that store customer data.

## Access control

- Access is granted on a need-to-know basis
- Reviews happen yearly
- Leavers lose access within 5 days

## Incident response

Incidents are reported to the security officer.

## Backups

Backups are made nightly and stored in a second region.

## Review

This policy is reviewed once a year by the management team.

| Level | Response time |
| ----- | ------------- |
| High  | 4 hours       |
| Low   | 3 days        |
`

const after = `# Information security policy

This policy applies to all **employees**, contractors and *suppliers* of Sourcehaven.

## Scope

The policy covers all systems that store customer data.

## Access control

- Access is granted on a need-to-know basis
- Reviews happen every six months
- Leavers lose access on their last day
- Privileged access requires a second approval

## Incident response

Incidents are reported to the security officer within 24 hours.

## Backups

Backups are made nightly and stored in a second region.

## Review

This policy is reviewed once a year by the management team.

| Level | Response time |
| ----- | ------------- |
| High  | 2 hours       |
| Low   | 3 days        |
`

const meta: Meta<typeof RlRenderedDiff> = {
  title: 'Doc/Rendered diff',
  component: RlRenderedDiff,
  parameters: { layout: 'padded' },
  args: { before, after, context: 1 },
}
export default meta
type Story = StoryObj<typeof RlRenderedDiff>

export const PolicyRevision: Story = {}
export const MoreContext: Story = { args: { context: 3 } }
export const NoChanges: Story = { args: { after: before } }
export const AddedAndRemovedBlocks: Story = {
  args: {
    before: 'Intro paragraph.\n\nTo be removed.\n\nOutro.',
    after: 'Intro paragraph.\n\nBrand new section with `code`.\n\nOutro.',
  },
}
export const UnsafeHtmlIsEscaped: Story = {
  args: {
    before: 'Hello',
    after: 'Hello <img src=x onerror=alert(1)> [x](javascript:alert(1))',
  },
}
