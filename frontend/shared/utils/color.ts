// Small sRGB helpers for deriving theme tokens and keeping text readable.

export const isHex = (value: unknown): value is string =>
  typeof value === 'string' && /^#[\da-f]{6}$/i.test(value)

export function rgb(hex: string): [number, number, number] {
  return [1, 3, 5].map((offset) => parseInt(hex.slice(offset, offset + 2), 16)) as [
    number,
    number,
    number,
  ]
}

const toHex = (values: number[]) =>
  '#' +
  values
    .map((v) =>
      Math.round(Math.max(0, Math.min(255, v)))
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')

/** Mix `b` into `a`; `amount` 0 returns `a`, 1 returns `b`. */
export function mix(a: string, b: string, amount: number) {
  const x = rgb(a)
  const y = rgb(b)
  return toHex(x.map((value, i) => value + (y[i]! - value) * amount))
}

export function luminance(hex: string) {
  const values = rgb(hex).map((channel) => {
    const value = channel / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  return values[0]! * 0.2126 + values[1]! * 0.7152 + values[2]! * 0.0722
}

export function contrast(a: string, b: string) {
  const x = luminance(a)
  const y = luminance(b)
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05)
}

/** Moves a colour towards white (dark backgrounds) or black (light ones) until it reaches `ratio`. */
export function readableOn(hex: string, background: string, ratio = 4.5) {
  const target = luminance(background) < 0.4 ? '#ffffff' : '#000000'
  for (let step = 0; step <= 20; step++) {
    const color = mix(hex, target, step / 20)
    if (contrast(color, background) >= ratio) return color
  }
  return target
}

/** Dark or light ink for text placed on a filled `hex` background. */
export function inkFor(hex: string) {
  return contrast(hex, '#111114') >= contrast(hex, '#ffffff')
    ? mix(hex, '#000000', 0.82)
    : '#ffffff'
}
