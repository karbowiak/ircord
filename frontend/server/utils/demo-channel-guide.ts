import type { components } from '../../shared/generated/api'
import type { DemoSession } from './demo-store'
import { addConversation } from './demo-store'
import { listChannels } from './irc-mock'

type GuideView = components['schemas']['ChannelGuideView']
type JoinResult = components['schemas']['ChannelGuideJoinResult']

/**
 * The demo's channel guide (#258): Libera Chat plays this server's own network, so Join
 * existing shows Featured there. Other networks have no guide, as with a real backend.
 */
const demoNetwork = 'libera'
const groups = [{ name: 'Programming' }, { name: 'Off topic' }]
const entries: Omit<GuideView['entries'][number], 'members' | 'topic'>[] = [
  {
    channel: '#linux',
    state: 'everyone',
    description: 'Help and news for every distribution',
    nsfw: false,
  },
  {
    channel: '#rust',
    group: 'Programming',
    state: 'suggested',
    description: 'Borrow checker support group',
    nsfw: false,
  },
  { channel: '#python', group: 'Programming', state: 'listed', nsfw: false },
  {
    channel: '#go-nuts',
    group: 'Programming',
    state: 'listed',
    description: 'Gophers welcome',
    nsfw: false,
  },
  {
    channel: '#photography',
    group: 'Off topic',
    state: 'suggested',
    description: 'Share your shots',
    nsfw: false,
  },
  { channel: '#weekend-plans', group: 'Off topic', state: 'listed', nsfw: true },
]

export function demoChannelGuide(session: DemoSession, networkId: string): GuideView {
  if (networkId !== demoNetwork) return { available: false, revision: 0, groups: [], entries: [] }
  const listing = new Map(
    listChannels(session, networkId, '', 100).map((c) => [c.name.toLowerCase(), c]),
  )
  return {
    available: true,
    revision: 1,
    groups,
    entries: entries.map((e) => {
      const fact = listing.get(e.channel.toLowerCase())
      return fact ? { ...e, members: fact.users, topic: fact.topic } : e
    }),
  }
}

export function demoJoinFromGuide(
  session: DemoSession,
  networkId: string,
  channels: string[],
): JoinResult {
  const offered = new Set(
    demoChannelGuide(session, networkId).entries.map((e) => e.channel.toLowerCase()),
  )
  const result: JoinResult = { joined: [], queued: [], failed: [] }
  for (const channel of new Set(channels.map((c) => c.toLowerCase()))) {
    if (!offered.has(channel)) throw new Error(`${channel} isn’t in the channel guide.`)
    result.joined.push(
      addConversation(session, { networkId, name: channel }) as JoinResult['joined'][number],
    )
  }
  return result
}
