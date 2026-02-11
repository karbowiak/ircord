<template>
  <div v-if="appStore.whoisState.open" class="overlay" @click.self="appStore.closeWhoisModal()">
    <div class="modal" role="dialog" aria-label="WHOIS result">
      <div class="header">
        <h2 class="title">WHOIS {{ appStore.whoisState.nick }}</h2>
        <button type="button" class="close" @click="appStore.closeWhoisModal()">x</button>
      </div>

      <div v-if="appStore.whoisState.loading" class="state">Loading WHOIS...</div>
      <div v-else-if="appStore.whoisState.error" class="state error">{{ appStore.whoisState.error }}</div>
      <div v-else-if="appStore.whoisState.info" class="grid">
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
.overlay {
  position: fixed;
  inset: 0;
  z-index: 95;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal {
  width: min(640px, 100%);
  max-height: min(80vh, 100%);
  overflow-y: auto;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(44, 48, 56, 0.98), rgba(36, 40, 46, 0.98));
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.45);
  padding: 14px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 16px;
  color: var(--text-primary);
}

.close {
  border: 0;
  border-radius: 7px;
  padding: 6px 9px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  font-family: var(--font-mono);
  cursor: pointer;
}

.grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.row {
  display: grid;
  grid-template-columns: 108px minmax(0, 1fr);
  gap: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  padding: 6px 0;
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
