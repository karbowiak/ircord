<script setup lang="ts">
const chat = useChatStore()
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
    if (join) await chat.join.mutateAsync({ networkId: current.networkId, name: current.channel })
    else await chat.rejectInvitation.mutateAsync(current.id)
  } catch (cause) {
    error.value = errorText(cause)
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div
    v-if="invitation"
    class="invitation-banner flex items-center gap-2.5 px-4 py-2 shrink-0 text-[13px] text-muted-foreground bg-card border-b border-b-border [border-bottom-style:solid] [&amp;_strong]:text-foreground [&amp;_strong]:font-[550]"
    role="status"
    :aria-label="$t('invitation.label')"
  >
    <AppIcon
      name="hash"
      :size="14"
    />
    <span class="invitation-copy flex-1 min-w-0 wrap-anywhere">
      <i18n-t
        keypath="invitation.message"
        tag="span"
      >
        <template #inviter
          ><strong>{{ invitation.inviterNick }}</strong></template
        >
        <template #channel
          ><strong>{{ invitation.channel }}</strong></template
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
      {{ $t('join.action') }}
    </UiButton>
    <UiButton
      variant="ghost"
      size="sm"
      class="text-foreground text-[13px]"
      :disabled="busy || chat.status !== 'connected'"
      :title="$t('invitation.rejectHelp')"
      @click="respond(false)"
    >
      {{ $t('invitation.reject') }}
    </UiButton>
  </div>
</template>
