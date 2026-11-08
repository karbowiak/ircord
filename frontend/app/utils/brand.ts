export interface BrandGlyph {
  path: string
  strokeWidth: number
  viewBox: string
}

// Share of the icon box the stroked glyph spans: Lucide's 20-of-24 live area, so the mark is the
// same size as the Lucide icons it sits beside.
const FILL = 5 / 6

const NUMBER = /-?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?/y
const COMMAND = /[A-Za-z]/y

// Extracts the hash glyph from the favicon SVG and frames it in a square viewBox centred on the
// glyph, so it can be drawn on its own at icon sizes. The favicon's path uses straight-line
// commands only (M, L, H, V, Z and their relative forms); anything else throws, so an artwork
// change this cannot follow fails the build instead of shipping a wrong or invisible icon.
export function brandGlyph(svg: string): BrandGlyph {
  const element = /<path\b[^>]*>/.exec(svg)?.[0]
  const path = element && /\sd="([^"]+)"/.exec(element)?.[1]
  if (!path) throw new Error('favicon.svg has no path')
  // The stroke width is set on the path or on the group around it.
  const before = svg.slice(0, svg.indexOf(element))
  const group = [...before.matchAll(/<g\b[^>]*>/g)].at(-1)?.[0] ?? ''
  const strokeWidth = Number(
    /stroke-width="([^"]+)"/.exec(element)?.[1] ?? /stroke-width="([^"]+)"/.exec(group)?.[1] ?? 1,
  )

  const xs: number[] = []
  const ys: number[] = []
  let x = 0
  let y = 0
  let startX = 0
  let startY = 0
  let command = ''
  let i = 0
  const skip = () => {
    while (i < path.length && /[\s,]/.test(path[i]!)) i++
  }
  const number = () => {
    skip()
    NUMBER.lastIndex = i
    const match = NUMBER.exec(path)
    if (!match) throw new Error(`favicon.svg path: expected a number at ${i}`)
    i = NUMBER.lastIndex
    return Number(match[0])
  }
  for (skip(); i < path.length; skip()) {
    COMMAND.lastIndex = i
    const letter = COMMAND.exec(path)?.[0]
    if (letter) {
      if (!'MmLlHhVvZz'.includes(letter))
        throw new Error(`favicon.svg path: unsupported command ${letter}`)
      command = letter
      i++
      if (command === 'Z' || command === 'z') {
        x = startX
        y = startY
      }
      continue
    }
    if (!command || command === 'Z' || command === 'z')
      throw new Error(`favicon.svg path: number without a command at ${i}`)
    const relative = command === command.toLowerCase()
    const op = command.toUpperCase()
    if (op === 'H') x = (relative ? x : 0) + number()
    else if (op === 'V') y = (relative ? y : 0) + number()
    else {
      x = (relative ? x : 0) + number()
      y = (relative ? y : 0) + number()
      if (op === 'M') {
        startX = x
        startY = y
        command = relative ? 'l' : 'L'
      }
    }
    xs.push(x)
    ys.push(y)
  }

  const half = strokeWidth / 2
  const minX = Math.min(...xs) - half
  const maxX = Math.max(...xs) + half
  const minY = Math.min(...ys) - half
  const maxY = Math.max(...ys) + half
  const side = Math.max(maxX - minX, maxY - minY) / FILL
  const round = (n: number) => Math.round(n * 100) / 100
  const box = [(minX + maxX - side) / 2, (minY + maxY - side) / 2, side, side].map(round)
  if (!box.every(Number.isFinite) || !(strokeWidth > 0))
    throw new Error('favicon.svg path: could not measure the glyph')
  return { path, strokeWidth, viewBox: box.join(' ') }
}
