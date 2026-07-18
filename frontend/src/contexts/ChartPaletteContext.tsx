import { createContext, useContext, useCallback, useState, type ReactNode } from 'react'
import type { PaletteId, ChartPalette } from '../lib/chartPalette'
import { CHART_PALETTES, loadPaletteId, savePaletteId } from '../lib/chartPalette'

interface ChartPaletteContextValue {
  paletteId: PaletteId
  palette: ChartPalette
  setPaletteId: (id: PaletteId) => void
}

const ChartPaletteContext = createContext<ChartPaletteContextValue | null>(null)

export function ChartPaletteProvider({ children }: { children: ReactNode }) {
  const [paletteId, setPaletteIdState] = useState<PaletteId>(() => loadPaletteId())

  const setPaletteId = useCallback((id: PaletteId) => {
    setPaletteIdState(id)
    savePaletteId(id)
  }, [])

  return (
    <ChartPaletteContext.Provider value={{ paletteId, palette: CHART_PALETTES[paletteId], setPaletteId }}>
      {children}
    </ChartPaletteContext.Provider>
  )
}

export function useChartPalette(): ChartPaletteContextValue {
  const ctx = useContext(ChartPaletteContext)
  if (!ctx) throw new Error('useChartPalette must be used within a ChartPaletteProvider')
  return ctx
}
