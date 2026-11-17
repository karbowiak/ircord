import type { CssReport, RemovalReason } from '../../../../shared/theme-css'

type Translate = (key: string, values?: Record<string, unknown>, plural?: number) => string

const order: RemovalReason[] = [
  'too-long',
  'limit',
  'load',
  'import',
  'font',
  'escape',
  'layer',
  'rule-type',
]

/** What the custom CSS filter removed, one plain sentence per reason. */
export function cssRemovalSummary(report: CssReport, t: Translate) {
  const counts = new Map<RemovalReason, { count: number; where: string }>()
  for (const removal of report.removed) {
    const entry = counts.get(removal.reason)
    if (entry) entry.count++
    else counts.set(removal.reason, { count: 1, where: removal.where })
  }
  const lines = order
    .filter((reason) => counts.has(reason))
    .map((reason) => {
      const { count, where } = counts.get(reason)!
      return t(`themes.removed.${reason}`, { count, where }, count)
    })
  if (report.unparsed)
    lines.push(t('themes.removed.unparsed', { count: report.unparsed }, report.unparsed))
  return lines
}
