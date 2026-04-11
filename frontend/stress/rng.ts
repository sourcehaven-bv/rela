// Tiny seedable RNG. mulberry32 — small, fast, deterministic, plenty
// good enough for picking random ops from a workload distribution.

import type { Rng } from './types.js'

export function makeRng(seed: number): Rng {
  let state = seed >>> 0
  function next(): number {
    state = (state + 0x6d2b79f5) >>> 0
    let t = state
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
  return {
    next,
    int(max: number): number {
      if (max <= 0) throw new Error('Rng.int requires max > 0')
      return Math.floor(next() * max)
    },
    pick<T>(items: readonly T[]): T {
      if (items.length === 0) throw new Error('Rng.pick on empty array')
      return items[Math.floor(next() * items.length)]!
    },
    pickWeighted(items: readonly { weight: number }[]): number {
      let total = 0
      for (const it of items) total += it.weight
      if (total <= 0) throw new Error('Rng.pickWeighted: total weight must be > 0')
      let r = next() * total
      for (let i = 0; i < items.length; i++) {
        r -= items[i]!.weight
        if (r <= 0) return i
      }
      return items.length - 1
    },
  }
}
