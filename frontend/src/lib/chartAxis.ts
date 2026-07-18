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
  if (!Number.isFinite(maxValue) || maxValue <= 0) return [0]
  const step = niceNumber(maxValue / tickCount)
  const ticks: number[] = []
  for (let v = 0; v <= maxValue + step / 2; v += step) {
    ticks.push(Math.round(v * 100) / 100)
  }
  return ticks
}
