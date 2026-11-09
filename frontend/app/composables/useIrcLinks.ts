import { networkNameFromHost, parseIrcUrl, type IrcLink } from '../../shared/utils/irc-url'
import { channelTarget } from '../../shared/utils/irc'
import { serverViewId } from '../stores/chat'

/**
 * Opens irc:// and ircs:// links: on a network you already have, join the channel (or open the
 * DM / server window); otherwise offer to add the network, prefilled. Also registers IRCord as
 * the browser's handler for these links.
 */
export function useIrcLinks() {
  const { t } = useI18n()
  const chat = useChatStore()
  const actions = useWorkspaceActions()

  /** Same server, or the same network under another hostname (irc.libera.chat ~ libera.chat). */
  function networkFor(link: IrcLink) {
    const base = (host: string) => networkNameFromHost(host.toLowerCase())
    return chat.data?.networks.find(
      (n) => n.host.toLowerCase() === link.host || base(n.host) === base(link.host),
    )
  }

  async function open(value: string | IrcLink) {
    const link = typeof value === 'string' ? parseIrcUrl(value) : value
    if (!link) return chat.notify(t('workspaceUi.commands.invalidIrcLink'))
    const network = networkFor(link)
    if (!network) {
      actions.addNetwork({
        name: networkNameFromHost(link.host),
        host: link.host,
        port: link.port,
        tls: link.tls,
        autoJoin: link.channel ? [link.key ? `${link.channel} ${link.key}` : link.channel] : [],
      })
      return
    }
    try {
      if (link.channel) {
        const existing = chat.data?.conversations.find(
          (c) =>
            c.networkId === network.id &&
            c.kind === 'channel' &&
            channelTarget(c).toLowerCase() === link.channel!.toLowerCase(),
        )
        if (existing) chat.activate(existing.id)
        else {
          chat.selectNetwork(network.id)
          await chat.join.mutateAsync({ name: link.channel, key: link.key, networkId: network.id })
        }
      } else if (link.nick) {
        const person = chat.memberByNick(link.nick)
        if (!person)
          return chat.notify(t('workspaceUi.commands.personNotAround', { nick: link.nick }))
        chat.selectNetwork(network.id)
        await chat.direct.mutateAsync(person.id)
      } else chat.activate(serverViewId(network.id))
    } catch (error) {
      chat.notify(errorText(error, t))
    }
  }

  return { open }
}

/** Asks the browser to send irc:// and ircs:// links to IRCord. Needs a user gesture. */
export function registerIrcHandler() {
  const register = navigator.registerProtocolHandler?.bind(navigator)
  if (!register) return false
  try {
    register('irc', `${location.origin}/?irc=%s`)
    register('ircs', `${location.origin}/?irc=%s`)
    return true
  } catch {
    return false
  }
}
