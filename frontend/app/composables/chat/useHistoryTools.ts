import { useQueryCache } from '@pinia/colada'
import type { Message, MessagePage } from '../../../shared/types/chat'
import { formatLog, logTime } from '../../../shared/utils/log'
import { messagesQuery } from '../../queries/chat'

const PAGE = 100
const MAX_EXPORT = 20_000

/**
 * Exporting a conversation as a text log, jumping back to a date in its history, and opening a
 * conversation at one message.
 */
export function useHistoryTools() {
  const { t, n, locale } = useI18n()
  const translateEvent = (
    key: string,
    values?: Record<string, string | number>,
    plural?: number,
  ) => (values ? (plural === undefined ? t(key, values) : t(key, values, plural)) : t(key))
  const chat = useChatStore()
  const queryCache = useQueryCache()
  const { ask } = usePrompt()
  const busy = ref(false)

  const label = (conversationId: string) => {
    const c = chat.data?.conversations.find((x) => x.id === conversationId)
    if (!c) return ''
    return c.kind === 'channel' ? `#${c.name}` : (chat.directPartner(c)?.nick ?? c.name)
  }

  /** Pages back through history (newest first) until `stop` says so or it runs out. */
  async function collect(conversationId: string, stop: (oldest: Message) => boolean) {
    const pages: Message[][] = []
    let before: string | undefined
    let total = 0
    do {
      const page = await apiClient().messages(conversationId, {
        limit: PAGE,
        ...(before ? { before } : {}),
      })
      pages.unshift(page.items)
      total += page.items.length
      before = page.nextCursor ?? undefined
      if (!page.items.length || stop(page.items[0]!)) break
    } while (before && total < MAX_EXPORT)
    return pages.flat()
  }

  async function exportLog(conversationId: string) {
    if (busy.value) return
    busy.value = true
    chat.notify(t('workspaceUi.history.preparing'))
    try {
      const messages = await collect(conversationId, () => false)
      const name = label(conversationId)
      const network = chat.data?.conversations.find((c) => c.id === conversationId)
      const networkName = network ? chat.networkOf(network)?.name : ''
      const text = formatLog(
        messages,
        (id) => chat.member(id)?.nick ?? id,
        t('workspaceUi.history.logHeading', {
          conversation: name,
          network: networkName ?? '',
          time: logTime(new Date().toISOString()),
        }),
        translateEvent,
        locale.value,
      )
      const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = `${(networkName || 'irc').toLowerCase().replace(/\W+/g, '-')}_${name.replace(/^#/, '')}_${new Date().toISOString().slice(0, 10)}.log`
      link.click()
      URL.revokeObjectURL(link.href)
      chat.notify(t('workspaceUi.history.exported', { count: n(messages.length) }, messages.length))
    } catch (error) {
      chat.notify(errorText(error, t))
    } finally {
      busy.value = false
    }
  }

  /**
   * Loads older history until it reaches `date`, then highlights the first message from that
   * day. The timeline keeps the most recent 500 messages, so very old dates may be out of reach.
   */
  async function jumpToDate(conversationId: string, highlight: (messageId: string) => void) {
    const value = await ask({
      title: t('workspaceUi.history.jumpTitle', { conversation: label(conversationId) }),
      label: t('workspaceUi.history.date'),
      value: new Date().toISOString().slice(0, 10),
      confirm: t('workspaceUi.history.jump'),
    })
    if (!value) return
    const target = new Date(`${value}T00:00:00`)
    if (Number.isNaN(target.getTime())) return chat.notify(t('workspaceUi.history.invalidDate'))
    chat.activate(conversationId)
    const page = () => queryCache.getQueryData<MessagePage>(messagesQuery(conversationId).key)
    for (let i = 0; i < 20; i++) {
      const current = page()
      const oldest = current?.items[0]
      if (!current?.nextCursor || (oldest && new Date(oldest.createdAt) <= target)) break
      const before = current.items.length
      await chat.loadOlder(conversationId)
      if ((page()?.items.length ?? 0) <= before) break
    }
    const items = page()?.items ?? []
    const first = items.find((m) => new Date(m.createdAt) >= target)
    if (!first) return chat.notify(t('workspaceUi.history.nothingOnDate'))
    if (new Date(items[0]!.createdAt) > target && page()?.nextCursor)
      chat.notify(t('workspaceUi.history.oldestLoaded'))
    highlight(first.id)
  }

  /**
   * Opens the conversation at one message. A message already in the timeline is simply shown;
   * otherwise the timeline becomes a bounded window of history around it, however far back it
   * is, until you jump to the present. Resolves whether it is there to highlight; otherwise it
   * has already said why. Opening never joins a channel you left.
   */
  async function reveal(conversationId: string, messageId: string) {
    if (!chat.data?.conversations.some((c) => c.id === conversationId)) {
      chat.notify(t('workspaceUi.history.conversationClosed'))
      return false
    }
    chat.activate(conversationId)
    const page = () => queryCache.getQueryData<MessagePage>(messagesQuery(conversationId).key)
    if (page()?.items.some((m) => m.id === messageId)) return true
    try {
      await chat.openAround(conversationId, messageId)
    } catch (error) {
      chat.notify(
        (error as { status?: number }).status === 404
          ? t('workspaceUi.history.messageGone')
          : errorText(error, t),
      )
      return false
    }
    return !!page()?.items.some((m) => m.id === messageId)
  }

  return { busy, exportLog, jumpToDate, reveal }
}
