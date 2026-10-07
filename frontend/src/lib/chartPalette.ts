export type PaletteId = 'vivid' | 'pastel' | 'okabeIto'

export interface ChartPalette {
  income: string
  expenses: string
  surplus: string
  // Fixed slot-per-position colors for widgets comparing up to 8 entities
  // (categories, years) — never cycled, identity comes from position.
  // Slots 0-3 are byte-identical to the original 4-slot palette so existing
  // widgets (capped at MAX_CATEGORY_SLOTS = 4 in trendsDashboard.ts) render
  // unchanged; slots 4-7 are reserved for widgets that need more series.
  categorical: [string, string, string, string, string, string, string, string]
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
    categorical: ['teal.6', 'blue.6', 'grape.6', 'orange.6', 'violet.6', 'cyan.6', 'pink.6', 'lime.6'],
  },
  pastel: {
    income: 'teal.4',
    expenses: 'red.4',
    surplus: 'blue.4',
    categorical: ['teal.4', 'blue.4', 'grape.4', 'orange.4', 'violet.4', 'cyan.4', 'pink.4', 'lime.4'],
  },
  okabeIto: {
    income: '#009E73',
    expenses: '#D55E00',
    surplus: '#0072B2',
    // Slots 4-6 are the three remaining chromatic colors of the 8-colour
    // Okabe-Ito set (bluish green, vermillion, blue — also used above for
    // income/expenses/surplus, which never share a chart with a categorical
    // series). The set's 8th color is black, which reads as axis/text ink, and
    // its gray fails the chroma floor (it renders as "no series color"), so
    // slot 7 is Tol's wine #882255 — the validator passes every CVD check on
    // the resulting 8-slot sequence.
    categorical: ['#E69F00', '#56B4E9', '#CC79A7', '#F0E442', '#009E73', '#D55E00', '#0072B2', '#882255'],
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

// Palette entries are either a Mantine token ('teal.6') or a raw hex value
// (the Okabe-Ito set) — CSS needs the token resolved to its variable.
export function paletteColorValue(color: string): string {
  return color.includes('.') ? `var(--mantine-color-${color.replace('.', '-')})` : color
}
