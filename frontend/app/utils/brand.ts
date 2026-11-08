export interface BrandGlyph {
  path: string
  strokeWidth: number
  viewBox: string
}

// Share of the icon box the glyph spans, matching the Lucide icons it sits beside (16 of 24).
const FILL = 2 / 3

// Extracts the hash glyph from the favicon SVG and frames it in a square viewBox centred on the
// glyph, so it can be drawn on its own at icon sizes. The favicon's path uses only straight-line
// commands (M, L, H, V and their relative forms), which is all this reads.
export function brandGlyph(svg: string): BrandGlyph {
  const path = /<path[^>]*\sd="([^"]+)"/.exec(svg)?.[1]
  const strokeWidth = Number(/stroke-width="([\d.]+)"/.exec(svg)?.[1] ?? 2)
  if (!path) throw new Error('favicon.svg has no path')
  const tokens = path.match(/[MLHVmlhv]|-?\d*\.?\d+/g) ?? []
  let x = 0
  let y = 0
  let command = 'M'
  const xs: number[] = []
  const ys: number[] = []
  for (let i = 0; i < tokens.length;) {
    const token = tokens[i]!
    if (/[A-Za-z]/.test(token)) {
      command = token
      i++
      continue
    }
    const relative = command === command.toLowerCase()
    const op = command.toUpperCase()
    if (op === 'H') x = (relative ? x : 0) + Number(tokens[i++])
    else if (op === 'V') y = (relative ? y : 0) + Number(tokens[i++])
    else {
      x = (relative ? x : 0) + Number(tokens[i++])
      y = (relative ? y : 0) + Number(tokens[i++])
      if (op === 'M') command = relative ? 'l' : 'L'
    }
    xs.push(x)
    ys.push(y)
  }
  const minX = Math.min(...xs) - strokeWidth / 2
  const maxX = Math.max(...xs) + strokeWidth / 2
  const minY = Math.min(...ys) - strokeWidth / 2
  const maxY = Math.max(...ys) + strokeWidth / 2
  const side = Math.max(maxX - minX, maxY - minY) / FILL
  const round = (n: number) => Math.round(n * 100) / 100
  const left = round((minX + maxX - side) / 2)
  const top = round((minY + maxY - side) / 2)
  return { path, strokeWidth, viewBox: `${left} ${top} ${round(side)} ${round(side)}` }
}
