<template>
  <div v-if="isOpen" class="modal-overlay" @click.self="emit('close')">
    <div class="modal-panel" role="dialog" aria-modal="true" aria-label="Connect to IRC server">
      <h2 class="modal-title">Connect to server</h2>

      <form class="form" @submit.prevent="onSubmit">
        <label>
          <span>Network name</span>
          <input v-model="name" type="text" placeholder="Ergo Local" required>
        </label>

        <label>
          <span>WebSocket URL</span>
          <input v-model="webSocketUrl" type="text" placeholder="ws://localhost:8067" required>
        </label>

        <label>
          <span>Server name</span>
          <input v-model="serverName" type="text" placeholder="localhost" required>
        </label>

        <div class="row">
          <label>
            <span>Nick</span>
            <input v-model="nick" type="text" required>
          </label>
          <label>
            <span>Username (optional)</span>
            <input v-model="username" type="text" placeholder="defaults to nick">
          </label>
        </div>

        <label>
          <span>Real name (optional)</span>
          <input v-model="realname" type="text" placeholder="IRCord User">
        </label>

        <label>
          <span>Password (optional)</span>
          <input v-model="password" type="password" autocomplete="current-password">
        </label>

        <label>
          <span>Channels (comma-separated)</span>
          <input v-model="channelsText" type="text" placeholder="#general,#ircord">
        </label>

        <p v-if="errorText" class="error-text">{{ errorText }}</p>

        <div class="actions">
          <button type="button" class="btn-secondary" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn-primary" :disabled="isSubmitting">
            {{ isSubmitting ? 'Connecting...' : 'Connect' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  isOpen: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const appStore = useAppStore()
const runtimeConfig = useRuntimeConfig()

const name = ref('Ergo Local')
const webSocketUrl = ref(String(runtimeConfig.public.ircWebSocketUrl || 'ws://localhost:8067'))
const serverName = ref(String(runtimeConfig.public.ircServerName || 'localhost'))
const nick = ref(String(runtimeConfig.public.ircNick || 'ircord'))
const username = ref('')
const realname = ref(String(runtimeConfig.public.ircRealname || 'IRCord User'))
const password = ref('')
const channelsText = ref('')
const isSubmitting = ref(false)
const errorText = ref('')

watch(() => props.isOpen, (open) => {
  if (open) {
    errorText.value = ''
  }
})

async function onSubmit() {
  if (isSubmitting.value) return

  isSubmitting.value = true
  errorText.value = ''

  try {
    const channels = channelsText.value
      .split(',')
      .map(part => part.trim())
      .filter(Boolean)

    const connected = await appStore.connectIrcServer({
      name: name.value,
      webSocketUrl: webSocketUrl.value,
      serverName: serverName.value,
      nick: nick.value,
      username: username.value,
      realname: realname.value,
      password: password.value,
      channels,
    })

    if (!connected) {
      errorText.value = 'Unable to connect. Check server URL and credentials.'
      return
    }

    emit('close')
  } catch (err) {
    errorText.value = 'Unable to connect. Check server URL and credentials.'
    console.error('[ircord] Connect failed:', err)
  } finally {
    isSubmitting.value = false
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.62);
  display: grid;
  place-items: center;
  z-index: 1000;
}

.modal-panel {
  width: min(560px, calc(100vw - 24px));
  background: var(--bg-darker);
  border: 1px solid var(--border-color);
  border-radius: 14px;
  padding: 18px;
}

.modal-title {
  margin: 0 0 14px;
  color: var(--text-primary);
  font-size: 1.05rem;
}

.form {
  display: grid;
  gap: 12px;
}

label {
  display: grid;
  gap: 6px;
}

span {
  color: var(--text-muted);
  font-size: 0.85rem;
}

input {
  width: 100%;
  height: 34px;
  border: 1px solid var(--border-color);
  background: var(--bg-darkest);
  color: var(--text-primary);
  border-radius: 8px;
  padding: 0 10px;
}

.row {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr 1fr;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.btn-secondary,
.btn-primary {
  border-radius: 8px;
  border: 1px solid var(--border-color);
  height: 34px;
  padding: 0 12px;
  cursor: pointer;
}

.btn-secondary {
  background: var(--bg-darkest);
  color: var(--text-primary);
}

.btn-primary {
  background: var(--accent);
  color: #fff;
  border-color: transparent;
}

.btn-primary:disabled {
  opacity: 0.7;
  cursor: default;
}

.error-text {
  margin: 0;
  color: #f87171;
  font-size: 0.85rem;
}

@media (max-width: 640px) {
  .row {
    grid-template-columns: 1fr;
  }
}
</style>
