// Recharts sometimes adds the exact data max as an extra y-axis tick on top
// of its own "nice number" ticks, producing an unevenly-spaced last gap
// (e.g. 0, 450, 900, 1800 instead of 0, 450, 900, 1350, 1800) — most visible
// on short/flat datasets. Computing our own evenly-spaced ticks and passing
// them explicitly avoids that entirely.
function niceNumber(value: number): number {
  if (value <= 0) return 0
  const exponent = Math.floor(Math.log10(value))
  const fraction = value / 10 ** exponent
  let niceFraction: number
  if (fraction <= 1) niceFraction = 1
  else if (fraction <= 2) niceFraction = 2
  else if (fraction <= 5) niceFraction = 5
  else niceFraction = 10
  return niceFraction * 10 ** exponent
}

export function niceAxisTicks(maxValue: number, tickCount = 4): number[] {
  return niceAxisTicksSigned(0, maxValue, tickCount)
}

// Like niceAxisTicks, but also extends the axis below zero when minValue is
// negative — a month where expenses exceed income (the surplus worth
// seeing) would otherwise render flattened onto the baseline, since every
// surplus chart previously passed a hardcoded domain of [0, max].
export function niceAxisTicksSigned(minValue: number, maxValue: number, tickCount = 4): number[] {
  const safeMax = Number.isFinite(maxValue) ? Math.max(maxValue, 0) : 0
  const safeMin = Number.isFinite(minValue) ? Math.min(minValue, 0) : 0
  if (safeMax === 0 && safeMin === 0) return [0]

  const step = niceNumber((safeMax - safeMin) / tickCount)
  if (step <= 0) return [0]

  const ticks: number[] = []
  const start = Math.floor(safeMin / step) * step
  for (let v = start; v <= safeMax + step / 2; v += step) {
    ticks.push(Math.round(v * 100) / 100)
  }
  return ticks
}

// Recharts' own x-axis auto-thinning picks an uneven subset once 8-12+
// category ticks don't fit (dropping just "Nov" while keeping every other
// month), so every chart with a month-sized x-axis passes this explicitly.
export function categoryTickInterval(pointCount: number): number {
  return Math.max(0, Math.ceil(pointCount / 6) - 1)
}
