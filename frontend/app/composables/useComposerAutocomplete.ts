import { useQuery } from '@pinia/colada'
import type { Ref, WritableComputedRef } from 'vue'
import type { Member } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText, rankEmoji, searchEmoji } from '../../shared/utils/media'
import { commands, type CommandSpec } from './useCommands'
import { channelListQuery } from '../queries/chat'
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

  const caret = ref(0)
  const index = ref(0)
  const dismissedAt = ref(-1)
  const token = computed(() => {
    if (dismissedAt.value === caret.value) return null
    const before = draft.value.slice(0, caret.value)
    const command = /^\/(\w*)$/.exec(before)
    if (command) return { kind: 'command' as const, query: command[1]! }
    const match = /(?:^|\s)(?::([\w+-]{2,32})|@([\w[\]\\^{}|`-]{0,32})|#([^\s,#:]{0,49}))$/.exec(
      before,
    )
    if (!match) return null
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
  let channelTimer: ReturnType<typeof setTimeout> | undefined
  watch(
    () => (token.value?.kind === 'channel' ? token.value.query : null),
    (q) => {
      clearTimeout(channelTimer)
      if (q === null) return
      channelTimer = setTimeout(() => (channelQuery.value = q), 200)
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
      ? commands.filter(
          (c) =>
            c.name.startsWith(token.value!.query.toLowerCase()) &&
            (!c.channel || chat.active?.kind === 'channel'),
        )
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
        tier: m.nick.toLowerCase().startsWith(q)
          ? 0
          : m.displayName.toLowerCase().startsWith(q)
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
          a.m.nick.localeCompare(b.m.nick),
      )
      .map((x) => x.m)
  }
  /** Members of this conversation: nick prefix first, then name matches; online before away. */
  const members = computed(() =>
    token.value?.kind === 'member' ? matchingMembers(token.value.query).slice(0, 12) : [],
  )
  const count = computed(() =>
    token.value?.kind === 'member'
      ? members.value.length
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
    command: (command: CommandSpec) => replaceToken(`/${command.name} `),
    channel: (channel: ChannelSuggestion) => replaceToken(`#${channel.name} `),
  }
  function completeActive() {
    const kind = token.value?.kind
    if (kind === 'member') complete.member(members.value[index.value]!)
    else if (kind === 'channel') complete.channel(channels.value[index.value]!)
    else if (kind === 'command') complete.command(commandList.value[index.value]!)
    else complete.emoji(emoji.value[index.value]!)
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

  // IRC-style Tab completion: “ju⇥” → “jules: ” at the start of a line, “jules ” elsewhere.
  // Repeated Tab cycles through matches.
  let tabCycle: { start: number; matches: string[]; index: number; end: number } | null = null
  function tabComplete() {
    const before = draft.value.slice(0, caret.value)
    if (tabCycle && tabCycle.end === caret.value) {
      tabCycle.index = (tabCycle.index + 1) % tabCycle.matches.length
    } else {
      const word = /(\S+)$/.exec(before)?.[1]
      if (!word) return false
      const matches = matchingMembers(word.replace(/^@/, ''))
        .filter((m) => m.nick.toLowerCase().startsWith(word.replace(/^@/, '').toLowerCase()))
        .map((m) => m.nick)
      if (!matches.length) return false
      tabCycle = { start: before.length - word.length, matches, index: 0, end: 0 }
    }
    const nick = tabCycle.matches[tabCycle.index]!
    const text = tabCycle.start === 0 ? `${nick}: ` : `${nick} `
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
    suggestions: { emoji, members, commands: commandList, channels },
    complete,
    trackCaret,
    onInput,
    keydown,
    dismiss,
    reset,
  }
}
