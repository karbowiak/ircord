// The custom CSS filter for personal themes (#88). It never reads the raw text for safety: the
// browser parses it into a detached stylesheet, the filter walks the parsed rules, and only what
// it serialises again is used. Escapes are resolved by then. Runs in the browser at apply time,
// every time, so a stricter filter in a later release also covers themes saved earlier.
import { themeTokens } from './schemes'
import { byteLength, themeLimits } from './themes'

export type RemovalReason =
  /** A value that would load an outside file: url(), image-set() and friends. */
  | 'load'
  /** @import, which would load another stylesheet. */
  | 'import'
  /** @font-face; themes can name fonts installed on the device. */
  | 'font'
  /** A rule type themes can't use, such as @page or @counter-style. */
  | 'rule-type'
  /** A backslash where the browser keeps the raw text (custom properties, var()). */
  | 'escape'
  /** More than 1,000 rules, keyframe frames included. */
  | 'limit'
  /** @layer: its rules are kept without the layer; layer statements are removed. */
  | 'layer'
  /** More than 64 KiB of CSS. */
  | 'too-long'

export interface CssRemoval {
  reason: RemovalReason
  /** The selector or at-rule it was in, shortened. */
  where: string
}
export interface CssReport {
  removed: CssRemoval[]
  /** Rules the browser could not parse at all; it drops them silently. */
  unparsed: number
  rules: number
  /** Known appearance variables the CSS sets on the root, so Appearance can say so. */
  rootVariables: string[]
}
export interface FilteredCss {
  css: string
  report: CssReport
}

export const cssRuleLimit = 1000

/** Every documented variable a theme may rely on (docs/theming.md). */
export const themeVariables = [
  ...themeTokens.map((token) => `--${token}`),
  '--font-ui',
  '--font-message',
  '--font-mono',
  '--message-size',
  '--group-gap',
  '--radius',
  '--radius-large',
  '--interface-scale',
]

// Any function that can fetch: url() and its spellings, image functions, element() and paint().
const loading =
  /(?:^|[^\w-])(?:-webkit-|-moz-)?(?:url|image-set|image|src|element|paint|cross-fade|canvas)\s*\(/i
const unparsedValue = /\b(?:var|env|attr)\s*\(/i
const rootSelector = /^\s*(?::root|html)(?::root)*\s*$/i

/** Splits a serialised declaration block at its top-level semicolons. */
function declarations(block: string) {
  const items: string[] = []
  let depth = 0
  let quote = ''
  let start = 0
  for (let i = 0; i < block.length; i++) {
    const c = block[i]!
    if (quote) {
      if (c === '\\') i++
      else if (c === quote) quote = ''
    } else if (c === '"' || c === "'") quote = c
    else if (c === '(' || c === '[') depth++
    else if (c === ')' || c === ']') depth = Math.max(0, depth - 1)
    else if (c === ';' && depth === 0) {
      items.push(block.slice(start, i))
      start = i + 1
    }
  }
  items.push(block.slice(start))
  return items.map((item) => item.trim()).filter(Boolean)
}

/** Top-level blocks and statements in the raw text, for the "could not parse" count only. */
function topLevelCount(text: string) {
  const plain = text.replace(/\/\*[\s\S]*?(?:\*\/|$)/g, '')
  let depth = 0
  let quote = ''
  let count = 0
  let pending = false
  for (let i = 0; i < plain.length; i++) {
    const c = plain[i]!
    if (quote) {
      if (c === '\\') i++
      else if (c === quote) quote = ''
      continue
    }
    if (c === '"' || c === "'") quote = c
    else if (c === '{') {
      if (depth === 0) count++
      depth++
      pending = false
    } else if (c === '}') depth = Math.max(0, depth - 1)
    else if (depth === 0 && c === ';') {
      if (pending) count++
      pending = false
    } else if (depth === 0 && c === '@') pending = true
  }
  return count
}

const short = (value: string) => {
  const flat = value.replace(/\s+/g, ' ').trim()
  return flat.length > 80 ? `${flat.slice(0, 77)}…` : flat
}
const kind = (rule: CSSRule) => rule.constructor?.name ?? ''
const head = (rule: CSSRule) => {
  const text = rule.cssText
  const brace = text.indexOf('{')
  return (brace < 0 ? text : text.slice(0, brace)).trim()
}

interface Walk {
  removed: CssRemoval[]
  rules: number
  rootVariables: Set<string>
}

function safeDeclarations(style: CSSStyleDeclaration, where: string, walk: Walk, root: boolean) {
  const kept: string[] = []
  for (const item of declarations(style.cssText)) {
    const colon = item.indexOf(':')
    if (colon <= 0) continue
    const name = item.slice(0, colon).trim()
    const value = item.slice(colon + 1)
    if (loading.test(value)) {
      walk.removed.push({ reason: 'load', where })
      continue
    }
    if (value.includes('\\') && (name.startsWith('--') || unparsedValue.test(value))) {
      walk.removed.push({ reason: 'escape', where })
      continue
    }
    if (root && themeVariables.includes(name)) walk.rootVariables.add(name)
    kept.push(`${name}:${value}`)
  }
  return kept.join(';')
}

/** `parentRoot`: these rules are nested in a style rule that targets the root element. */
function rules(list: CSSRuleList, walk: Walk, parentRoot = false): string {
  const out: string[] = []
  for (const rule of Array.from(list)) {
    const css = ruleText(rule, walk, parentRoot)
    if (css) out.push(css)
  }
  return out.join('\n')
}

/** Whether a selector list sets values on the root element (`html, body`, or `&` under `:root`). */
function targetsRoot(selector: string, parentRoot: boolean) {
  return selector
    .split(',')
    .some((part) => rootSelector.test(part) || (parentRoot && /^\s*&\s*$/.test(part)))
}

function ruleText(rule: CSSRule, walk: Walk, parentRoot: boolean): string {
  if (walk.rules >= cssRuleLimit) {
    if (!walk.removed.some((r) => r.reason === 'limit'))
      walk.removed.push({ reason: 'limit', where: short(head(rule)) })
    return ''
  }
  walk.rules++
  const type = kind(rule)
  const grouping = (rule as CSSGroupingRule).cssRules
  const where = short(head(rule))
  switch (type) {
    case 'CSSStyleRule': {
      const style = rule as CSSStyleRule
      const selector = style.selectorText
      const root = targetsRoot(selector, parentRoot)
      const body = safeDeclarations(style.style, short(selector), walk, root)
      const nested = style.cssRules ? rules(style.cssRules, walk, root) : ''
      return `${selector}{${body}${nested ? `\n${nested}` : ''}}`
    }
    case 'CSSNestedDeclarations':
      return safeDeclarations((rule as CSSStyleRule).style, where, walk, parentRoot)
    case 'CSSMediaRule':
    case 'CSSSupportsRule':
    case 'CSSContainerRule':
    case 'CSSLayerBlockRule':
    case 'CSSStartingStyleRule': {
      // Rebuilt from the parsed condition rather than cut out of the rule's text.
      const at = rule as CSSRule & {
        media?: MediaList
        conditionText?: string
        containerName?: string
        containerQuery?: string
        name?: string
      }
      const prelude =
        type === 'CSSMediaRule'
          ? `@media ${at.media!.mediaText}`
          : type === 'CSSSupportsRule'
            ? `@supports ${at.conditionText}`
            : type === 'CSSContainerRule'
              ? `@container ${[at.containerName, at.containerQuery].filter(Boolean).join(' ')}`
              : ''
      // Layer names are shared with the app's own layers, and the order of first use decides
      // which layer wins. Theme CSS stays unlayered, so the app's layered !important rules (the
      // display and reduced-motion guarantees) always win, wherever this stylesheet lands.
      if (type === 'CSSLayerBlockRule') {
        walk.removed.push({ reason: 'layer', where })
        return rules(grouping, walk, parentRoot)
      }
      if (type === 'CSSStartingStyleRule')
        return `@starting-style{${rules(grouping, walk, parentRoot)}}`
      if (loading.test(prelude) || /[\\{};]/.test(prelude)) {
        walk.removed.push({ reason: 'load', where })
        return ''
      }
      return `${prelude.trim()}{${rules(grouping, walk, parentRoot)}}`
    }
    case 'CSSLayerStatementRule':
      // Declaring layer order could reorder the app's own layers.
      walk.removed.push({ reason: 'layer', where })
      return ''
    case 'CSSKeyframesRule': {
      const keyframes = rule as CSSKeyframesRule
      const frames: string[] = []
      for (const frame of Array.from(keyframes.cssRules)) {
        // Each frame counts toward the rule limit, like any other rule.
        if (walk.rules >= cssRuleLimit) {
          if (!walk.removed.some((r) => r.reason === 'limit'))
            walk.removed.push({ reason: 'limit', where })
          break
        }
        walk.rules++
        const key = (frame as CSSKeyframeRule).keyText
        frames.push(
          `${key}{${safeDeclarations((frame as CSSKeyframeRule).style, where, walk, false)}}`,
        )
      }
      return `@keyframes ${CSS.escape(keyframes.name)}{${frames.join('')}}`
    }
    case 'CSSPropertyRule': {
      const text = rule.cssText
      if (loading.test(text) || text.includes('\\')) {
        walk.removed.push({ reason: 'load', where })
        return ''
      }
      return text
    }
    case 'CSSImportRule':
      walk.removed.push({ reason: 'import', where })
      return ''
    case 'CSSFontFaceRule':
      walk.removed.push({ reason: 'font', where })
      return ''
    default:
      walk.removed.push({ reason: 'rule-type', where })
      return ''
  }
}

function pass(text: string): FilteredCss {
  const walk: Walk = { removed: [], rules: 0, rootVariables: new Set() }
  const sheet = new CSSStyleSheet()
  // replaceSync drops @import rules itself; count them for the report.
  const imports = (text.replace(/\/\*[\s\S]*?(?:\*\/|$)/g, '').match(/@import\b/gi) ?? []).length
  for (let i = 0; i < imports; i++) walk.removed.push({ reason: 'import', where: '@import' })
  try {
    sheet.replaceSync(text)
  } catch {
    return {
      css: '',
      report: { removed: walk.removed, unparsed: 1, rules: 0, rootVariables: [] },
    }
  }
  const css = rules(sheet.cssRules, walk)
  return {
    css,
    report: {
      removed: walk.removed,
      unparsed: Math.max(0, topLevelCount(text) - sheet.cssRules.length - imports),
      rules: walk.rules,
      rootVariables: [...walk.rootVariables],
    },
  }
}

/**
 * The CSS that may be applied, and a report of what was removed. The output is parsed and checked
 * once more; if that check finds anything, nothing is applied.
 */
export function filterThemeCss(text: string): FilteredCss {
  if (byteLength(text) > themeLimits.css)
    return {
      css: '',
      report: {
        removed: [{ reason: 'too-long', where: '' }],
        unparsed: 0,
        rules: 0,
        rootVariables: [],
      },
    }
  if (!text.trim())
    return { css: '', report: { removed: [], unparsed: 0, rules: 0, rootVariables: [] } }
  const first = pass(text)
  if (first.css && pass(first.css).report.removed.some((r) => r.reason !== 'limit'))
    return { css: '', report: first.report }
  return first
}
