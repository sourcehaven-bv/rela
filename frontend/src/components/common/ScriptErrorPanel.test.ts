import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import ScriptErrorPanel from './ScriptErrorPanel.vue'
import type { ScriptError } from '../../types/scriptError'

describe('ScriptErrorPanel', () => {
  it('names only the line when the failure is in the script itself', () => {
    const error: ScriptError = {
      error: 'script_error',
      script: { surface: 'action', path: 'actions/a.lua' },
      lua: { message: 'boom', line: 2 },
    }
    const text = mount(ScriptErrorPanel, { props: { error } }).text()
    expect(text).toContain('line 2:')
    expect(text).not.toContain('lib/')
  })

  it('names the module file when the failure is in a required module', () => {
    const error: ScriptError = {
      error: 'script_error',
      script: { surface: 'action', path: 'actions/a.lua' },
      lua: { message: 'inside module', line: 3, file: 'lib/fail.lua' },
    }
    const text = mount(ScriptErrorPanel, { props: { error } }).text()
    expect(text).toContain('lib/fail.lua line 3:')
    expect(text).toContain('actions/a.lua')
  })
})
