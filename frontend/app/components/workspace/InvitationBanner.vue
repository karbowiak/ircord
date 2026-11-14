<script setup lang="ts" vapor>
import { errorText } from '../../utils/api'
const chat = useChatStore()
const { t, n: $n } = useI18n()
const $t = t
const invitation = computed(() => chat.data?.invitations?.[0])
const remaining = computed(() => Math.max(0, (chat.data?.invitations?.length ?? 0) - 1))
const busy = ref(false)
const error = ref('')
watch(
  () => invitation.value?.id,
  () => {
    error.value = ''
  },
)
async function respond(join: boolean) {
  const current = invitation.value
  if (!current || busy.value) return
  busy.value = true
  error.value = ''
  try {
    if (join && (!chat.data?.demo || current.party)) await chat.acceptInvitation(current.id)
    else if (join)
      await chat.join.mutateAsync({ networkId: current.networkId, name: current.channel })
    else await chat.rejectInvitation.mutateAsync(current.id)
  } catch (cause) {
    error.value = errorText(cause, (key) => t(key))
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div
    v-if="invitation"
    class="invitation-banner flex items-center gap-2.5 px-4 py-2 shrink-0 text-[13px] text-muted-foreground bg-card border-b border-b-border [border-bottom-style:solid] [&_strong]:text-foreground [&_strong]:font-[550]"
    role="status"
    :aria-label="$t('invitation.label')"
  >
    <AppIcon
      :name="invitation.party ? 'users' : 'hash'"
      :size="14"
    />
    <span class="invitation-copy flex-1 min-w-0 wrap-anywhere">
      <i18n-t
        :keypath="invitation.party ? 'workspaceUi.party.invitation' : 'invitation.message'"
        tag="span"
      >
        <template #inviter
          ><strong>{{ invitation.inviterNick }}</strong></template
        >
        <template #channel
          ><strong>{{
            invitation.party
              ? invitation.party.name || $t('workspaceUi.party.unnamed')
              : invitation.channel
          }}</strong></template
        >
        <template #network
          ><span class="invitation-network text-muted-foreground">{{
            chat.data?.networks.find((n) => n.id === invitation?.networkId)?.name ?? 'IRC'
          }}</span></template
        >
      </i18n-t>
      <span
        v-if="remaining"
        class="invitation-network text-muted-foreground ms-[.3em]"
      >
        · {{ $t('invitation.more', { count: $n(remaining) }, remaining) }}</span
      >
      <span
        v-if="error"
        class="invitation-error block text-destructive"
        >{{ error }}</span
      >
    </span>
    <UiButton
      variant="ghost"
      size="sm"
      class="text-foreground text-[13px]"
      :disabled="busy || chat.status !== 'connected'"
      @click="respond(true)"
    >
      {{ invitation.party ? $t('workspaceUi.party.accept') : $t('join.action') }}
    </UiButton>
    <UiButton
      variant="ghost"
      size="sm"
      class="text-foreground text-[13px]"
      :disabled="busy || chat.status !== 'connected'"
      :title="invitation.party ? $t('workspaceUi.party.declineHelp') : $t('invitation.rejectHelp')"
      @click="respond(false)"
    >
      {{ invitation.party ? $t('workspaceUi.party.decline') : $t('invitation.reject') }}
    </UiButton>
  </div>
</template>
