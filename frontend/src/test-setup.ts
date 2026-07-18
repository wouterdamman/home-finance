import '@testing-library/jest-dom'

// window.localStorage comes back `undefined` in this jsdom+Node combo (Node
// 22+ ships its own inert global `localStorage`, which appears to stop
// vitest's jsdom environment from attaching the real implementation to
// `window`) — app code that calls `localStorage.getItem(...)` directly would
// silently hit a "Cannot read properties of undefined" error in every test.
// A minimal in-memory Storage polyfill sidesteps the interaction entirely.
class MemoryStorage implements Storage {
  private store = new Map<string, string>()
  get length() {
    return this.store.size
  }
  clear() {
    this.store.clear()
  }
  getItem(key: string) {
    return this.store.has(key) ? this.store.get(key)! : null
  }
  key(index: number) {
    return Array.from(this.store.keys())[index] ?? null
  }
  removeItem(key: string) {
    this.store.delete(key)
  }
  setItem(key: string, value: string) {
    this.store.set(key, String(value))
  }
}

const memoryStorage = new MemoryStorage()
Object.defineProperty(globalThis, 'localStorage', { value: memoryStorage, configurable: true })
Object.defineProperty(window, 'localStorage', { value: memoryStorage, configurable: true })
