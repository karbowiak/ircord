import type { Member } from '../../../shared/types/chat'
import { memberNameColor } from '../../../shared/utils/identity'

/** Inline style for a member's name; plain text colour when coloured names are turned off. */
export function useNameStyle() {
  const appearance = useAppearanceStore()
  return (member?: Member) =>
    appearance.settings.coloredNames || member?.nameColor
      ? { color: memberNameColor(member, appearance.mode, appearance.tokens.main) }
      : { color: 'var(--text)' }
}
