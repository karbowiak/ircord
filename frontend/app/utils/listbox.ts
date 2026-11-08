/** Where a popup goes: below its trigger, or above when there's more room there. */
export function placePopup(
  trigger: { top: number; bottom: number; left: number; width: number },
  viewport: { width: number; height: number },
  wanted: number,
  gap = 4,
  margin = 8,
) {
  const below = viewport.height - trigger.bottom - gap - margin
  const above = trigger.top - gap - margin
  const up = below < Math.min(wanted, 160) && above > below
  const maxHeight = Math.max(0, Math.min(wanted, up ? above : below))
  const width = Math.min(Math.max(trigger.width, 160), viewport.width - margin * 2)
  const left = Math.max(margin, Math.min(trigger.left, viewport.width - margin - width))
  return up
    ? { up, left, width, maxHeight, bottom: viewport.height - trigger.top + gap }
    : { up, left, width, maxHeight, top: trigger.bottom + gap }
}
