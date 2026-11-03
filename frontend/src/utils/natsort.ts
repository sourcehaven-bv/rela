/**
 * Natural string order, ported from the server's `internal/natsort`: digit
 * runs compare by value, text compares case-insensitively with case as the
 * last tie-break, and a digit sorts before text. The SPA uses it where it
 * must pick the same item the server picks from a sorted list. Like the Go
 * version it folds ASCII letters only.
 */
export function naturalCompare(a: string, b: string): number {
  let ia = 0
  let ib = 0
  let caseTiebreak = 0
  while (ia < a.length && ib < b.length) {
    const ca = a.charCodeAt(ia)
    const cb = b.charCodeAt(ib)
    const digitA = isDigit(ca)
    const digitB = isDigit(cb)
    if (digitA && digitB) {
      const zerosA = countRun(a, ia, isZero)
      const zerosB = countRun(b, ib, isZero)
      ia += zerosA
      ib += zerosB
      const lenA = countRun(a, ia, isDigit)
      const lenB = countRun(b, ib, isDigit)
      if (lenA !== lenB) return lenA < lenB ? -1 : 1
      const digitsA = a.slice(ia, ia + lenA)
      const digitsB = b.slice(ib, ib + lenB)
      if (digitsA !== digitsB) return digitsA < digitsB ? -1 : 1
      if (zerosA !== zerosB) return zerosA < zerosB ? -1 : 1
      ia += lenA
      ib += lenB
    } else if (!digitA && !digitB) {
      const lowA = toLower(ca)
      const lowB = toLower(cb)
      if (lowA !== lowB) return lowA < lowB ? -1 : 1
      if (caseTiebreak === 0 && ca !== cb) caseTiebreak = ca < cb ? -1 : 1
      ia++
      ib++
    } else {
      return digitA ? -1 : 1
    }
  }
  const remA = a.length - ia
  const remB = b.length - ib
  if (remA !== remB) return remA < remB ? -1 : 1
  return caseTiebreak
}

function isDigit(c: number): boolean {
  return c >= 48 && c <= 57
}

function isZero(c: number): boolean {
  return c === 48
}

function toLower(c: number): number {
  return c >= 65 && c <= 90 ? c + 32 : c
}

function countRun(s: string, start: number, match: (c: number) => boolean): number {
  let i = start
  while (i < s.length && match(s.charCodeAt(i))) i++
  return i - start
}
