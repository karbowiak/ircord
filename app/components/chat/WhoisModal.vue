<template>
  <AppModal :open="appStore.whoisState.open" aria-label="WHOIS result" size="md" @close="appStore.closeWhoisModal()">
    <div class="modal">
      <div class="modal-header">
        <div>
          <h2 class="modal-title">WHOIS {{ appStore.whoisState.nick }}</h2>
          <p class="modal-subtitle">Identity and session details for this user.</p>
        </div>
        <button type="button" class="close" @click="appStore.closeWhoisModal()">x</button>
      </div>

      <div v-if="appStore.whoisState.loading" class="state">Loading WHOIS...</div>
      <div v-else-if="appStore.whoisState.error" class="state error">{{ appStore.whoisState.error }}</div>
      <div v-else-if="appStore.whoisState.info" class="section">
        <div class="grid">
          <div class="row"><span class="label">Nick</span><span class="value">{{ appStore.whoisState.info.nick }}</span></div>
          <div class="row"><span class="label">Username</span><span class="value">{{ appStore.whoisState.info.username || 'n/a' }}</span></div>
          <div class="row"><span class="label">Hostname</span><span class="value">{{ appStore.whoisState.info.hostname || 'n/a' }}</span></div>
          <div class="row"><span class="label">Realname</span><span class="value">{{ appStore.whoisState.info.realname || 'n/a' }}</span></div>
          <div class="row"><span class="label">Server</span><span class="value">{{ appStore.whoisState.info.server || 'n/a' }}</span></div>
          <div class="row"><span class="label">Server info</span><span class="value">{{ appStore.whoisState.info.serverInfo || 'n/a' }}</span></div>
          <div class="row"><span class="label">Account</span><span class="value">{{ appStore.whoisState.info.account || 'n/a' }}</span></div>
          <div class="row"><span class="label">Channels</span><span class="value">{{ appStore.whoisState.info.channels.join(' ') || 'n/a' }}</span></div>
          <div class="row"><span class="label">Operator</span><span class="value">{{ appStore.whoisState.info.isOperator ? 'yes' : 'no' }}</span></div>
          <div class="row"><span class="label">Secure</span><span class="value">{{ appStore.whoisState.info.secure ? 'yes' : 'no' }}</span></div>
          <div class="row"><span class="label">Idle</span><span class="value">{{ formatIdle(appStore.whoisState.info.idleSeconds) }}</span></div>
          <div class="row"><span class="label">Signon</span><span class="value">{{ formatDate(appStore.whoisState.info.signonTime) }}</span></div>
        </div>
      </div>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
const appStore = useAppStore()

function formatIdle(value: number | null): string {
  if (value === null || Number.isNaN(value)) return 'n/a'
  if (value < 60) return `${value}s`
  if (value < 3600) return `${Math.floor(value / 60)}m ${value % 60}s`
  return `${Math.floor(value / 3600)}h ${Math.floor((value % 3600) / 60)}m`
}

function formatDate(value: string | null): string {
  if (!value) return 'n/a'
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}
</script>

<style scoped>
.modal {
  width: 100%;
  max-height: min(80vh, 88vh);
  overflow-y: auto;
  display: grid;
  gap: 12px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
}

.modal-title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 18px;
  color: var(--text-primary);
}

.modal-subtitle {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.4;
}

.close {
  border: 0;
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  font-family: var(--font-mono);
  cursor: pointer;
}

.close:hover {
  background: rgba(255, 255, 255, 0.14);
  color: var(--text-primary);
}

.section {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
  padding: 12px;
}

.grid {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.row {
  display: grid;
  grid-template-columns: 108px minmax(0, 1fr);
  gap: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  padding: 7px 0;
}

.label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.value {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
  word-break: break-word;
}

.state {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
  padding: 12px;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
}

.state.error {
  color: #ff7d7d;
}

@media (max-width: 680px) {
  .row {
    grid-template-columns: 1fr;
  }
}
</style>
