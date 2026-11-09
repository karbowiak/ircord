import {
  channelWide,
  groupMentions,
  isGroupMention,
  mayReachChannel,
  statusAtLeast,
} from '../../shared/group-mentions'
import { useQuery } from '@pinia/colada'
import type { Ref, WritableComputedRef } from 'vue'
import type { Member } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText, rankEmoji, searchEmoji } from '../../shared/utils/media'
import { commands, type CommandSpec } from './useCommands'
import { channelListQuery } from '../queries/chat'
import { matchCommands } from '../../shared/utils/commands'
import { channelRights } from '../../shared/utils/irc'
import type { ChannelSuggestion } from '../utils/suggestions'

interface Options {
  draft: WritableComputedRef<string>
  input: Ref<HTMLTextAreaElement | null>
  rawMode: Ref<boolean>
  /** Replaces the draft and puts the caret at `position`. */
  setDraft: (value: string, position: number) => void
}

const presence = { online: 0, away: 1, offline: 2 }

/**
 * Composer autocomplete: “/” commands, “:ke” emoji, “@ec” people and “#de” channels, plus
 * IRC-style Tab nick completion. “:joy:” turns into 😂 as the closing colon is typed.
 */
export function useComposerAutocomplete({ draft, input, rawMode, setDraft }: Options) {
  const chat = useChatStore()
  const library = useLibraryStore()
  const media = useMediaStore()

  const isMemberNick = (nick: string) =>
    !!nick &&
    (chat.active?.memberIds ?? []).some(
      (id) => chat.member(id)?.nick.toLowerCase() === nick.toLowerCase(),
    )
  const caret = ref(0)
  const index = ref(0)
  const dismissedAt = ref(-1)
  const token = computed(() => {
    if (dismissedAt.value === caret.value) return null
    const before = draft.value.slice(0, caret.value)
    const command = /^\/(\w*)$/.exec(before)
    if (command) return { kind: 'command' as const, query: command[1]! }
    const match = /(?:^|\s)(?::([\w+-]{2,32})|@([^@\n]{0,100})|#([^\s,#:]{0,49}))$/.exec(before)
    if (!match) return null
    // “/unignore @ju” completes from the ignore list on Tab, not from people who are here.
    if (match[2] !== undefined && /^\/unignore\s+@\S*$/i.test(before)) return null
    // A finished mention (“@PeterPowers ” or “@here ” after Tab) is done: no list, so Enter sends.
    if (
      match[2] !== undefined &&
      /\s$/.test(match[2]) &&
      (isMemberNick(match[2].trim()) || isGroupMention(`@${match[2].trim()}`))
    )
      return null
    return match[1] !== undefined
      ? { kind: 'emoji' as const, query: match[1] }
      : match[2] !== undefined
        ? { kind: 'member' as const, query: match[2] }
        : { kind: 'channel' as const, query: match[3]! }
  })

  /**
   * “#de” suggests channels on this network: the ones you're in first (unread before read), then
   * matches from the network's channel list, so you can point people at channels you haven't joined.
   */
  const channelQuery = ref('')
  const { start: suggestChannels, stop: cancelChannels } = useTimeoutFn(
    (query: string) => (channelQuery.value = query),
    200,
    { immediate: false },
  )
  watch(
    () => (token.value?.kind === 'channel' ? token.value.query : null),
    (q) => {
      cancelChannels()
      if (q === null) return
      suggestChannels(q)
    },
  )
  const directory = useQuery(() => ({
    ...channelListQuery({ networkId: chat.networkId, q: channelQuery.value }),
    enabled: token.value?.kind === 'channel' && channelQuery.value.length > 0 && !rawMode.value,
  }))
  const channels = computed<ChannelSuggestion[]>(() => {
    if (token.value?.kind !== 'channel') return []
    const q = token.value.query.toLowerCase()
    const joined = (chat.data?.conversations ?? [])
      .filter((c) => c.networkId === chat.networkId && c.kind === 'channel')
      .filter((c) => c.name.toLowerCase().includes(q))
      .sort(
        (a, b) =>
          Number(!b.name.toLowerCase().startsWith(q)) -
            Number(!a.name.toLowerCase().startsWith(q)) ||
          Number(!!b.mentionCount) - Number(!!a.mentionCount) ||
          Number(!!b.unreadCount) - Number(!!a.unreadCount) ||
          a.name.localeCompare(b.name),
      )
      .map((c) => ({
        name: c.name,
        joined: true,
        users: c.memberIds.length,
        topic: c.topic,
        unread: c.mentionCount || c.unreadCount,
      }))
    const known = new Set(joined.map((c) => c.name.toLowerCase()))
    const listed = (directory.data.value?.items ?? [])
      .map((c) => ({
        name: c.name.replace(/^#/, ''),
        joined: false,
        users: c.users,
        topic: c.topic,
      }))
      .filter((c) => !known.has(c.name.toLowerCase()) && c.name.toLowerCase().includes(q))
    return [...joined, ...listed].slice(0, 12)
  })
  watch(
    () => token.value?.kind + ':' + token.value?.query,
    () => {
      index.value = 0
      if (token.value?.kind === 'emoji') media.needAll()
    },
  )
  const commandList = computed<CommandSpec[]>(() =>
    token.value?.kind === 'command'
      ? matchCommands(commands, token.value.query, {
          channel: chat.active?.kind === 'channel',
          operator: channelRights('you', chat.active, chat.network).op,
        })
      : [],
  )
  const emoji = computed(() =>
    token.value?.kind === 'emoji'
      ? rankEmoji(
          searchEmoji(media.emojis, token.value.query),
          library.usageRank,
          token.value.query,
        ).slice(0, 20)
      : [],
  )
  function matchingMembers(query: string) {
    const q = query.toLowerCase()
    return (chat.active?.memberIds ?? [])
      .map(chat.member)
      .filter((m): m is NonNullable<typeof m> => !!m && m.id !== 'you')
      .map((m) => ({
        m,
        tier: (m.displayName || m.nick).toLowerCase().startsWith(q)
          ? 0
          : m.nick.toLowerCase().startsWith(q)
            ? 1
            : `${m.nick} ${m.displayName}`.toLowerCase().includes(q)
              ? 2
              : 3,
      }))
      .filter((x) => x.tier < 3)
      .sort(
        (a, b) =>
          a.tier - b.tier ||
          presence[a.m.status] - presence[b.m.status] ||
          (a.m.displayName || a.m.nick).localeCompare(b.m.displayName || b.m.nick),
      )
      .map((x) => x.m)
  }
  /** Members of this conversation: display name first, then nickname matches; online before away. */
  const members = computed(() =>
    token.value?.kind === 'member' ? matchingMembers(token.value.query).slice(0, 12) : [],
  )
  /**
   * IRCord group mentions (@here, @operator…) after the people, in channels only. @here, @channel
   * and @everyone are left out when the channel's setting means yours would reach nobody.
   */
  const groups = computed(() => {
    const c = chat.active
    if (token.value?.kind !== 'member' || c?.kind !== 'channel') return []
    const q = token.value.query.toLowerCase()
    const wide = mayReachChannel(
      c.channelSettings?.groupMentions,
      statusAtLeast('you', c, chat.networkOf(c), 'o'),
    )
    return groupMentions.filter(
      (g) => g.name.startsWith(q) && (wide || !channelWide.includes(g.name)),
    )
  })
  const count = computed(() =>
    token.value?.kind === 'member'
      ? members.value.length + groups.value.length
      : token.value?.kind === 'command'
        ? commandList.value.length
        : token.value?.kind === 'channel'
          ? channels.value.length
          : emoji.value.length,
  )

  function trackCaret() {
    caret.value = input.value?.selectionStart ?? draft.value.length
  }
  /** Replaces the “/query”, “:query”, “@query” or “#query” before the caret. */
  function replaceToken(text: string) {
    const start = caret.value - (token.value?.query.length ?? 0) - 1
    setDraft(
      draft.value.slice(0, start) + text + draft.value.slice(caret.value),
      start + text.length,
    )
  }
  const complete = {
    emoji(item: EmojiItem) {
      const value = emojiText(item, library.library.skinTone)
      library.useEmoji(item)
      replaceToken(item.unicode ? value : `${value} `)
    },
    member: (member: Member) => replaceToken(`@${member.nick} `),
    group: (group: { name: string }) => replaceToken(`@${group.name} `),
    command: (command: CommandSpec) => replaceToken(`/${command.name} `),
    channel: (channel: ChannelSuggestion) => replaceToken(`#${channel.name} `),
  }
  function completeActive() {
    const kind = token.value?.kind
    if (kind === 'member')
      if (index.value < members.value.length) complete.member(members.value[index.value]!)
      else complete.group(groups.value[index.value - members.value.length]!)
    else if (kind === 'channel') complete.channel(channels.value[index.value]!)
    else if (kind === 'command') complete.command(commandList.value[index.value]!)
    else complete.emoji(emoji.value[index.value]!)
  }
  /** The highlighted suggestion is exactly what's already typed, so there is nothing to complete. */
  function activeIsTyped() {
    const query = token.value?.query.trim().toLowerCase()
    if (token.value?.kind === 'member')
      return (
        (
          members.value[index.value]?.nick ?? groups.value[index.value - members.value.length]?.name
        )?.toLowerCase() === query
      )
    if (token.value?.kind === 'channel')
      return channels.value[index.value]?.name.toLowerCase() === query
    return false
  }
  /** Typing the closing colon of a known Unicode shortcode converts it, like other chat apps. */
  function convertShortcode() {
    const match = /(?:^|\s):([\w+-]{2,32}):$/.exec(draft.value.slice(0, caret.value))
    const item = match && media.byShortcodeAll.get(match[1]!)
    if (!item?.unicode) return
    const start = caret.value - match![1]!.length - 2
    const value = emojiText(item, library.library.skinTone)
    library.useEmoji(item)
    setDraft(
      draft.value.slice(0, start) + value + draft.value.slice(caret.value),
      start + value.length,
    )
  }

  // Tab completion: “ju⇥” → “@jules: ” at the start of a line, “@jules ” elsewhere. The “@”
  // makes it a mention: IRCord highlights a nick only after “@” (#209). Repeated Tab cycles
  // through matches.
  let tabCycle: { start: number; matches: string[]; index: number; end: number } | null = null
  function tabComplete() {
    const before = draft.value.slice(0, caret.value)
    if (tabCycle && tabCycle.end === caret.value) {
      tabCycle.index = (tabCycle.index + 1) % tabCycle.matches.length
    } else {
      const word = /(\S+)$/.exec(before)?.[1]
      if (!word) return false
      const query = word.replace(/^@/, '')
      // “/unignore ju⇥” completes from the ignore list: ignored people are often not here.
      const matches = /^\/unignore\s+\S*$/i.test(before)
        ? useNotificationStore().settings.ignore.filter((mask) =>
            mask.toLowerCase().startsWith(query.toLowerCase()),
          )
        : matchingMembers(query).map((m) => m.nick)
      if (!matches.length) return false
      tabCycle = { start: before.length - word.length, matches, index: 0, end: 0 }
    }
    const nick = tabCycle.matches[tabCycle.index]!
    // Command arguments (“/msg ju⇥”, “/unignore ju⇥”) take the bare nick.
    const mention = before.startsWith('/') ? nick : `@${nick}`
    const text = tabCycle.start === 0 ? `${mention}: ` : `${mention} `
    const position = tabCycle.start + text.length
    tabCycle.end = position
    setDraft(draft.value.slice(0, tabCycle.start) + text + draft.value.slice(caret.value), position)
    return true
  }

  /** Call on every input event. */
  function onInput() {
    tabCycle = null
    trackCaret()
    convertShortcode()
  }
  /** Handles the keys autocomplete owns; returns true when the event was used. */
  function keydown(event: KeyboardEvent) {
    if (count.value && !event.isComposing) {
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault()
        index.value =
          (index.value + (event.key === 'ArrowDown' ? 1 : -1) + count.value) % count.value
        return true
      }
      // “@peterpowers” or “#general” typed out in full: Enter sends instead of completing it again.
      if (event.key === 'Enter' && !event.shiftKey && activeIsTyped()) return false
      if ((event.key === 'Enter' && !event.shiftKey) || event.key === 'Tab') {
        event.preventDefault()
        completeActive()
        return true
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        dismissedAt.value = caret.value
        return true
      }
    }
    if (event.key === 'Tab' && !event.shiftKey && !event.ctrlKey && !event.altKey) {
      trackCaret()
      if (tabComplete()) event.preventDefault()
      return true
    }
    return false
  }
  /** Leaving the field hides suggestions until the caret moves. */
  const dismiss = () => (dismissedAt.value = caret.value)
  const reset = () => (tabCycle = null)

  return {
    caret,
    token,
    index,
    count,
    suggestions: { emoji, members, groups, commands: commandList, channels },
    complete,
    trackCaret,
    onInput,
    keydown,
    dismiss,
    reset,
  }
}
