import type { Conversation } from '../../../shared/types/chat'
import { channelTarget } from '../../../shared/utils/irc'

/** Leaving a channel always asks first, from the conversation menu and from Channel settings. */
export function useLeaveChannel() {
  const { t } = useI18n()
  const chat = useChatStore()
  const { confirm } = usePrompt()
  /** Resolves to true when the person confirmed and the channel is being left. */
  return async (c: Conversation) => {
    const confirmed = await confirm({
      title: t('collectionsUi.channel.overview.leaveConfirm', { channel: channelTarget(c) }),
      confirm: t('collectionsUi.channel.overview.leave'),
      danger: true,
    })
    if (confirmed) chat.leave.mutate({ c })
    return confirmed
  }
}
