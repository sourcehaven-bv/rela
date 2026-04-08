// Scenario registry. Add a new scenario by importing it here.

import type { Scenario } from '../types.js'
import { watcherPressureScenario } from './watcher-pressure.js'

export const scenarios: Record<string, Scenario> = {
  [watcherPressureScenario.name]: watcherPressureScenario,
}
