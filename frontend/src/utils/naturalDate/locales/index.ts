import type { DatePack } from '../types'
import { en } from './en'
import { nl } from './nl'

// To add a language, write a pack next to en.ts and list it here. The shared
// table test in naturalDate.test.ts runs every listed pack.
export const DATE_PACKS: Readonly<Record<string, DatePack>> = { en, nl }

export const FALLBACK_PACK = en
