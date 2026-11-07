import type { Conversation, Network } from '../../shared/types/chat'
import { channelTarget } from '../../shared/utils/irc'
import { serverViewId } from '../stores/chat'

/** “Libera Chat” → “libera-chat”; ids keep it unique when two networks share a name. */
function slugs(networks: Network[]) {
  const base = (n: Network) =>
    n.name
      .toLowerCase()
      .normalize('NFKD')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '') || n.id
  const counts = new Map<string, number>()
  for (const n of networks) counts.set(base(n), (counts.get(base(n)) ?? 0) + 1)
  return new Map(
    networks.map((n) => [
      n.id,
      counts.get(base(n))! > 1 ? `${base(n)}-${n.id.slice(0, 6)}` : base(n),
    ]),
  )
}

/**
 * Keeps the address bar in step with what's open, so browser history works for conversations:
 * `#libera-chat/#general`, `#libera-chat/@jules`, and `#libera-chat` for the server window.
 * Switching pushes a history entry; back/forward (mouse buttons, gestures) switches back.
 * Call once from the workspace.
 */
export function useUrlSync() {
  const chat = useChatStore()
  const networkSlugs = computed(() => slugs(chat.data?.networks ?? []))

  function hashFor(id: string): string | null {
    const data = chat.data
    if (!data) return null
    if (id.startsWith('server:')) {
      const slug = networkSlugs.value.get(id.slice('server:'.length))
      return slug ? `#${slug}` : null
    }
    const c = data.conversations.find((x) => x.id === id)
    const slug = c && networkSlugs.value.get(c.networkId)
    if (!c || !slug) return null
    const target =
      c.kind === 'channel' ? channelTarget(c) : `@${chat.directPartner(c)?.nick ?? c.name}`
    return `#${slug}/${target}`
  }

  /** What a hash points at: a conversation id, or a DM to open with someone. */
  function resolve(
    hash: string,
  ): { id?: string; nick?: string; channel?: string; networkId?: string } | null {
    const value = decodeURIComponent(hash.replace(/^#/, ''))
    if (!value || !chat.data) return null
    const slash = value.indexOf('/')
    const slug = (slash < 0 ? value : value.slice(0, slash)).toLowerCase()
    const rest = slash < 0 ? '' : value.slice(slash + 1)
    const network = chat.data.networks.find(
      (n) => networkSlugs.value.get(n.id) === slug || n.id === slug,
    )
    if (!network) return null
    if (!rest) return { id: serverViewId(network.id) }
    const name = rest.slice(1).toLowerCase()
    const inNetwork = (c: Conversation) => c.networkId === network.id
    if (/^[#&+!*~]/.test(rest)) {
      const c = chat.data.conversations.find(
        (x) =>
          inNetwork(x) &&
          x.kind === 'channel' &&
          channelTarget(x).toLowerCase() === rest.toLowerCase(),
      )
      // A channel you're not in: join it, like an irc:// link (IRC creates it if it's empty).
      return c ? { id: c.id } : name ? { channel: rest, networkId: network.id } : null
    }
    if (rest.startsWith('@')) {
      const c = chat.data.conversations.find(
        (x) =>
          inNetwork(x) && x.kind === 'direct' && chat.directPartner(x)?.nick.toLowerCase() === name,
      )
      return c ? { id: c.id } : { nick: rest.slice(1), networkId: network.id }
    }
    return null
  }

  // Set while we apply a history entry, so the resulting switch doesn't push a new one.
  let restoring = false
  async function apply(hash: string) {
    const target = resolve(hash)
    if (!target) {
      if (hash && hash !== '#') chat.notify('That link points to a network you haven’t added.')
      return false
    }
    restoring = true
    try {
      if (target.id) chat.activate(target.id)
      else if (target.channel && target.networkId) {
        chat.selectNetwork(target.networkId)
        await chat.join.mutateAsync({ name: target.channel, networkId: target.networkId })
      } else if (target.nick && target.networkId) {
        const person = chat.memberByNick(target.nick)
        if (!person) {
          chat.notify(`No one called ${target.nick} is around.`)
          return false
        }
        chat.selectNetwork(target.networkId)
        await chat.direct.mutateAsync(person.id)
      }
      return true
    } catch (error) {
      chat.notify(errorText(error))
      return false
    } finally {
      await nextTick()
      restoring = false
    }
  }

  // Open what the URL names once the workspace has loaded; otherwise record where we are.
  const stopInitial = watch(
    () => chat.data,
    async (data) => {
      if (!data) return
      queueMicrotask(() => stopInitial())
      if (location.hash && (await apply(location.hash))) return
      const hash = hashFor(chat.activeId)
      if (hash) history.replaceState(history.state, '', hash)
    },
    { immediate: true },
  )

  // Every switch becomes a history entry (unless it came from history).
  watch(
    () => [chat.activeId, hashFor(chat.activeId)] as const,
    ([, hash]) => {
      if (
        !hash ||
        restoring ||
        location.hash === hash ||
        decodeURIComponent(location.hash) === hash
      )
        return
      history.pushState(null, '', hash)
    },
  )

  function popped() {
    void apply(location.hash)
  }
  // Chromium and Firefox turn mouse back/forward buttons into history navigation themselves;
  // Safari ignores them, so map them there (and only there, to avoid navigating twice).
  const safari = /^((?!chrome|chromium|android|crios|fxios|edg).)*safari/i.test(navigator.userAgent)
  function mouseButtons(event: MouseEvent) {
    if (event.button !== 3 && event.button !== 4) return
    event.preventDefault()
    if (event.button === 3) history.back()
    else history.forward()
  }
  window.addEventListener('popstate', popped)
  if (safari) window.addEventListener('mouseup', mouseButtons)
  onBeforeUnmount(() => {
    window.removeEventListener('popstate', popped)
    window.removeEventListener('mouseup', mouseButtons)
  })
}
