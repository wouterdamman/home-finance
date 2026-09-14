import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import nl from './locales/nl.json'

type Tree = { [key: string]: string | Tree }

function flatten(tree: Tree, prefix = ''): Map<string, string> {
  const out = new Map<string, string>()
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out.set(path, value)
    else for (const [k, v] of flatten(value, path)) out.set(k, v)
  }
  return out
}

const placeholders = (value: string) =>
  [...value.matchAll(/\{\{\s*([\w.]+)\s*\}\}/g)].map((m) => m[1]).sort()

const enFlat = flatten(en as Tree)
const nlFlat = flatten(nl as Tree)

// The locales were at exact parity and nothing enforced it, so a key added to
// one file only would ship as a raw key path in the other UI.
describe('locale parity', () => {
  it('defines the same keys in both languages', () => {
    const missingInNl = [...enFlat.keys()].filter((k) => !nlFlat.has(k))
    const missingInEn = [...nlFlat.keys()].filter((k) => !enFlat.has(k))
    expect({ missingInNl, missingInEn }).toEqual({ missingInNl: [], missingInEn: [] })
  })

  it('uses the same interpolation placeholders in both languages', () => {
    const mismatched = [...enFlat.entries()]
      .filter(([key, value]) => {
        const other = nlFlat.get(key)
        return other !== undefined && placeholders(value).join() !== placeholders(other).join()
      })
      .map(([key]) => key)
    expect(mismatched).toEqual([])
  })

  it('has no empty values', () => {
    const empty = [...enFlat.entries(), ...nlFlat.entries()]
      .filter(([, value]) => value.trim() === '')
      .map(([key]) => key)
    expect(empty).toEqual([])
  })
})
