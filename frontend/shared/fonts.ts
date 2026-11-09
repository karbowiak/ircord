import { interfaceFonts, messageFonts } from './appearance'

/** Register every saved appearance choice, including fonts absent from initial UI. */
export const fontFamilies = [
  ...new Set(
    [...Object.values(interfaceFonts), ...Object.values(messageFonts)]
      .map(({ stack }) => stack.match(/'([^']+)'/)?.[1])
      .filter((name): name is string => !!name && name.endsWith(' Variable')),
  ),
]
