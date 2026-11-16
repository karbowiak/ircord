import type { components } from '../generated/api'

export type GuideChannel = components['schemas']['ChannelGuideChannel']
export type GuideGroup = components['schemas']['ChannelGuideGroup']

export interface GuideSection {
  /** `everyone` lists channels every member joins; `group` an owner's group; `more` the rest. */
  kind: 'everyone' | 'group' | 'more'
  /** The owner's group name, for `group` sections. */
  name?: string
  channels: GuideChannel[]
}

export const guideKey = (channel: string) => channel.toLowerCase()

/**
 * The picker's layout (#258): channels for everyone first, then the owners' groups in their
 * order, then channels without a group. Groups without channels are left out.
 */
export function guideSections(groups: GuideGroup[], channels: GuideChannel[]): GuideSection[] {
  const everyone = channels.filter((c) => c.state === 'everyone')
  const rest = channels.filter((c) => c.state !== 'everyone')
  const known = new Set(groups.map((g) => g.name.toLowerCase()))
  const sections: GuideSection[] = []
  if (everyone.length) sections.push({ kind: 'everyone', channels: everyone })
  for (const group of groups) {
    const items = rest.filter((c) => c.group?.toLowerCase() === group.name.toLowerCase())
    if (items.length) sections.push({ kind: 'group', name: group.name, channels: items })
  }
  const ungrouped = rest.filter((c) => !c.group || !known.has(c.group.toLowerCase()))
  if (ungrouped.length) sections.push({ kind: 'more', channels: ungrouped })
  return sections
}

/**
 * Channels ticked when the picker opens: suggested channels you aren't in yet. Sensitive
 * channels never start ticked.
 */
export function initialGuideSelection(channels: GuideChannel[], joined: Set<string>): string[] {
  return channels
    .filter((c) => c.state === 'suggested' && !c.nsfw && !joined.has(guideKey(c.channel)))
    .map((c) => guideKey(c.channel))
}

/** Whether the guide offers anything to choose beyond the channels everyone joins. */
export function guideHasChoices(channels: GuideChannel[], joined: Set<string>): boolean {
  return channels.some((c) => c.state !== 'everyone' && !joined.has(guideKey(c.channel)))
}
