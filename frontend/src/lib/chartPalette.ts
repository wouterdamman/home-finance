export type PaletteId = 'vivid' | 'pastel' | 'okabeIto'

export interface ChartPalette {
  income: string
  expenses: string
  surplus: string
  // Fixed slot-per-position colors for widgets comparing up to 4 entities
  // (categories, years) — never cycled, identity comes from position.
  categorical: [string, string, string, string]
}

// "vivid" is the original palette this app shipped with — kept as the
// default so nobody's view changes unless they opt in.
// "pastel" softens the same hue families (lighter shade) for a calmer look.
// "okabeIto" is the Okabe-Ito colorblind-safe palette (Okabe & Ito, 2008),
// picked for users who want a palette validated for color-vision deficiency
// rather than just aesthetics.
export const CHART_PALETTES: Record<PaletteId, ChartPalette> = {
  vivid: {
    income: 'teal.6',
    expenses: 'red.6',
    surplus: 'blue.6',
    categorical: ['teal.6', 'blue.6', 'grape.6', 'orange.6'],
  },
  pastel: {
    income: 'teal.4',
    expenses: 'red.4',
    surplus: 'blue.4',
    categorical: ['teal.4', 'blue.4', 'grape.4', 'orange.4'],
  },
  okabeIto: {
    income: '#009E73',
    expenses: '#D55E00',
    surplus: '#0072B2',
    categorical: ['#E69F00', '#56B4E9', '#CC79A7', '#F0E442'],
  },
}

export const PALETTE_IDS: PaletteId[] = ['vivid', 'pastel', 'okabeIto']

const STORAGE_KEY = 'chart-palette'
const DEFAULT_PALETTE: PaletteId = 'vivid'

function isPaletteId(v: string | null): v is PaletteId {
  return v === 'vivid' || v === 'pastel' || v === 'okabeIto'
}

export function loadPaletteId(): PaletteId {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (isPaletteId(raw)) return raw
  } catch {
    // fall through to default
  }
  return DEFAULT_PALETTE
}

export function savePaletteId(id: PaletteId) {
  localStorage.setItem(STORAGE_KEY, id)
}
