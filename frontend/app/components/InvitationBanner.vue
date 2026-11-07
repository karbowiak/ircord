<script setup lang="ts">
const chat = useChatStore()
const invitation = computed(() => chat.data?.invitations?.[0])
const remaining = computed(() => Math.max(0, (chat.data?.invitations?.length ?? 0) - 1))
const busy = ref(false)
const error = ref('')
watch(() => invitation.value?.id, () => { error.value = '' })
async function respond(join: boolean) {
  const current = invitation.value
  if (!current || busy.value) return
  busy.value = true
  error.value = ''
  try {
    if (join) await chat.join.mutateAsync({ networkId: current.networkId, name: current.channel })
    else await chat.rejectInvitation.mutateAsync(current.id)
  } catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <div v-if="invitation" class="invitation-banner" role="status" aria-label="Channel invitation">
    <AppIcon name="hash" :size="14" />
    <span class="invitation-copy">
      <strong>{{ invitation.inviterNick }}</strong> invited you to
      <strong>{{ invitation.channel }}</strong>
      <span class="invitation-network">on {{ chat.data?.networks.find(n => n.id === invitation?.networkId)?.name ?? 'IRC' }}</span>
      <span v-if="remaining" class="invitation-network"> · {{ remaining }} more</span>
      <span v-if="error" class="invitation-error">{{ error }}</span>
    </span>
    <button :disabled="busy || chat.status !== 'connected'" @click="respond(true)">Join</button>
    <button :disabled="busy || chat.status !== 'connected'" title="Reject and block further invites to this channel" @click="respond(false)">Reject</button>
  </div>
</template>
<style scoped>
.invitation-banner { display:flex; align-items:center; gap:10px; padding:8px 16px; flex-shrink:0; font-size:12px; color:var(--muted); background:var(--panel); border-bottom:1px solid var(--border); }
.invitation-copy { flex:1; min-width:0; overflow-wrap:anywhere; }
strong { color:var(--text); font-weight:550; }
.invitation-network { color:var(--muted); margin-inline-start:.3em; }
button { color:var(--text); padding:4px 6px; border-radius:4px; font-size:12px; }
button:hover { background:var(--hover); }
button:disabled { opacity:.5; }
.invitation-error { display:block; color:var(--danger); }
</style>
