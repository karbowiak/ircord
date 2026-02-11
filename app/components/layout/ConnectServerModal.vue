<template>
  <AppModal
    :open="isOpen"
    aria-label="Connect to IRC server"
    :size="!isEditing && viewMode === 'directory' ? 'xl' : 'lg'"
    @close="emit('close')"
  >
    <div class="modal-panel">
      <div class="modal-header">
        <div>
          <h2 class="modal-title">{{ isEditing ? 'Edit server' : 'Connect to server' }}</h2>
          <p class="modal-subtitle">Configure connection details and channels for this network.</p>
        </div>
        <button type="button" class="close" @click="emit('close')">x</button>
      </div>

      <div v-if="!isEditing" class="mode-toggle">
        <button
          type="button"
          class="mode-btn"
          :class="{ active: viewMode === 'directory' }"
          @click="viewMode = 'directory'"
        >
          Server directory
        </button>
        <button
          type="button"
          class="mode-btn"
          :class="{ active: viewMode === 'custom' }"
          @click="viewMode = 'custom'"
        >
          Connect custom
        </button>
      </div>

      <section v-if="!isEditing && viewMode === 'directory'" class="section">
        <div class="directory-head">
          <h3 class="section-title">Recommended IRC networks</h3>
          <button type="button" class="btn-secondary" @click="viewMode = 'custom'">Connect to custom</button>
        </div>

        <div class="directory-table-wrap">
          <table class="directory-table">
            <thead>
              <tr>
                <th>Network</th>
                <th>Category</th>
                <th>Country</th>
                <th>Users</th>
                <th>Channels</th>
                <th>Ping</th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in ircServerDirectory" :key="entry.id">
                <td>
                  <div class="network-name">{{ entry.name }}</div>
                  <div class="network-host">{{ entry.serverName }}</div>
                </td>
                <td>{{ entry.category }}</td>
                <td>{{ entry.country }}</td>
                <td>{{ formatNumber(entry.users) }}</td>
                <td>{{ formatNumber(entry.channels) }}</td>
                <td>{{ entry.latencyMs }}ms</td>
                <td>
                  <button type="button" class="btn-primary table-action" @click="selectDirectoryServer(entry)">
                    Use
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <form v-if="isEditing || viewMode === 'custom'" class="form" @submit.prevent="onSubmit">
        <section class="section">
          <h3 class="section-title">Network</h3>

          <label class="field">
            <span class="field-label">Network name</span>
            <input v-model="name" type="text" placeholder="Ergo Local" required>
          </label>

          <label class="field">
            <span class="field-label">WebSocket URL</span>
            <input v-model="webSocketUrl" type="text" placeholder="ws://localhost:8067" required>
          </label>

          <label class="field">
            <span class="field-label">Server name</span>
            <input v-model="serverName" type="text" placeholder="localhost" required>
          </label>
        </section>

        <section class="section">
          <h3 class="section-title">Identity</h3>

          <div class="row">
            <label class="field">
              <span class="field-label">Nick</span>
              <input v-model="nick" type="text" required>
            </label>

            <label class="field">
              <span class="field-label">Username</span>
              <input v-model="username" type="text" required>
            </label>
          </div>

          <label class="field">
            <span class="field-label">Real name (optional)</span>
            <input v-model="realname" type="text" placeholder="IRCord User">
          </label>
        </section>

        <section class="section">
          <h3 class="section-title">Authentication</h3>

          <label class="field">
            <span class="field-label">Services account</span>
            <input v-model="servicesAccount" type="text" required>
          </label>

          <label class="field">
            <span class="field-label">Services password</span>
            <input v-model="servicesPassword" type="password" autocomplete="current-password" required>
          </label>

          <label class="field">
            <span class="field-label">Server password (optional)</span>
            <input v-model="serverPassword" type="password" autocomplete="current-password">
          </label>
        </section>

        <section class="section section-compact">
          <label class="field">
            <span class="field-label">Channels (comma-separated)</span>
            <input v-model="channelsText" type="text" placeholder="#general,#ircord">
          </label>

          <label class="toggle-row">
            <input v-model="autoOwnershipTake" type="checkbox">
            <span>Automatically claim ownership of empty channels where I join as sole operator</span>
          </label>
        </section>

        <p v-if="errorText" class="error-text">{{ errorText }}</p>

        <div class="actions">
          <button
            v-if="!isEditing"
            type="button"
            class="btn-secondary"
            @click="viewMode = 'directory'"
          >
            Back to directory
          </button>
          <button type="button" class="btn-secondary" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn-primary" :disabled="isSubmitting">
            {{ isSubmitting ? (isEditing ? 'Saving...' : 'Connecting...') : (isEditing ? 'Save' : 'Connect') }}
          </button>
        </div>
      </form>
    </div>
  </AppModal>
</template>

<script setup lang="ts">
import { ircServerDirectory } from '~/data/ircServerDirectory'

const props = defineProps<{
  isOpen: boolean
  editingServerId?: string | null
}>()

const emit = defineEmits<{
  close: []
}>()

const appStore = useAppStore()
const runtimeConfig = useRuntimeConfig()

const defaultName = 'Ergo Local'
const defaultWebSocketUrl = String(runtimeConfig.public.ircWebSocketUrl || 'ws://localhost:8067')
const defaultServerName = String(runtimeConfig.public.ircServerName || 'localhost')
const defaultNick = String(runtimeConfig.public.ircNick || 'ircord')
const defaultUsername = String(runtimeConfig.public.ircUsername || runtimeConfig.public.ircNick || 'ircord')
const defaultRealname = String(runtimeConfig.public.ircRealname || 'IRCord User')
const defaultServicesAccount = String(runtimeConfig.public.ircServicesAccount || runtimeConfig.public.ircNick || 'ircord')

const name = ref(defaultName)
const webSocketUrl = ref(defaultWebSocketUrl)
const serverName = ref(defaultServerName)
const nick = ref(defaultNick)
const username = ref(defaultUsername)
const realname = ref(defaultRealname)
const servicesAccount = ref(defaultServicesAccount)
const servicesPassword = ref('')
const serverPassword = ref('')
const autoOwnershipTake = ref(false)
const channelsText = ref('')
const isSubmitting = ref(false)
const errorText = ref('')
const viewMode = ref<'directory' | 'custom'>('directory')

const isEditing = computed(() => Boolean(props.editingServerId))

function resetFormDefaults() {
  name.value = defaultName
  webSocketUrl.value = defaultWebSocketUrl
  serverName.value = defaultServerName
  nick.value = defaultNick
  username.value = defaultUsername
  realname.value = defaultRealname
  servicesAccount.value = defaultServicesAccount
  servicesPassword.value = ''
  serverPassword.value = ''
  autoOwnershipTake.value = false
  channelsText.value = ''
}

function populateFormFromServer(serverId: string) {
  const config = appStore.getIrcServerConfig(serverId)
  if (!config) {
    resetFormDefaults()
    return
  }

  name.value = config.name
  webSocketUrl.value = config.webSocketUrl
  serverName.value = config.serverName
  nick.value = config.nick
  username.value = config.username
  realname.value = config.realname || ''
  servicesAccount.value = config.servicesAccount
  servicesPassword.value = config.servicesPassword
  serverPassword.value = config.serverPassword || ''
  autoOwnershipTake.value = Boolean(config.autoOwnershipTake)
  channelsText.value = config.channels.join(',')
}

watch(() => [props.isOpen, props.editingServerId] as const, ([open, editingServerId]) => {
  if (open) {
    errorText.value = ''
    viewMode.value = editingServerId ? 'custom' : 'directory'
    if (editingServerId) {
      populateFormFromServer(editingServerId)
    } else {
      resetFormDefaults()
    }
  }
})

function selectDirectoryServer(entry: (typeof ircServerDirectory)[number]) {
  name.value = entry.name
  webSocketUrl.value = entry.webSocketUrl
  serverName.value = entry.serverName
  channelsText.value = entry.suggestedChannels.join(',')
  viewMode.value = 'custom'
}

function formatNumber(value: number): string {
  return value.toLocaleString('en-US')
}

async function onSubmit() {
  if (isSubmitting.value) return

  isSubmitting.value = true
  errorText.value = ''

  try {
    const payload = {
      name: name.value,
      webSocketUrl: webSocketUrl.value,
      serverName: serverName.value,
      nick: nick.value,
      username: username.value,
      realname: realname.value,
      servicesAccount: servicesAccount.value,
      servicesPassword: servicesPassword.value,
      serverPassword: serverPassword.value,
      autoOwnershipTake: autoOwnershipTake.value,
      channels: channelsText.value
      .split(',')
      .map(part => part.trim())
      .filter(Boolean),
    }

    const connected = props.editingServerId
      ? await appStore.editIrcServer(props.editingServerId, payload)
      : await appStore.connectIrcServer(payload)

    if (!connected) {
      errorText.value = 'Unable to save server. Check server URL and credentials.'
      return
    }

    emit('close')
  } catch (err) {
    errorText.value = 'Unable to save server. Check server URL and credentials.'
    console.error('[ircord] Connect failed:', err)
  } finally {
    isSubmitting.value = false
  }
}
</script>

<style scoped>
.modal-panel {
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
  color: var(--text-primary);
  font-size: 18px;
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

.mode-toggle {
  display: inline-flex;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  overflow: hidden;
  width: fit-content;
}

.mode-btn {
  height: 34px;
  border: 0;
  padding: 0 12px;
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
  cursor: pointer;
}

.mode-btn.active {
  background: rgba(88, 101, 242, 0.22);
  color: var(--text-primary);
}

.directory-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.directory-table-wrap {
  overflow: auto;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
}

.directory-table {
  width: 100%;
  border-collapse: collapse;
  font-family: var(--font-mono);
  font-size: 12px;
  min-width: 680px;
}

.directory-table th,
.directory-table td {
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  padding: 9px 10px;
  text-align: left;
  color: var(--text-body);
}

.directory-table th {
  color: var(--text-muted);
  font-size: 11px;
  text-transform: uppercase;
}

.network-name {
  color: var(--text-primary);
}

.network-host {
  margin-top: 2px;
  color: var(--text-faint);
  font-size: 11px;
}

.table-action {
  height: 30px;
  border-radius: 8px;
  padding: 0 10px;
}

.form {
  display: grid;
  gap: 10px;
}

.section {
  display: grid;
  gap: 11px;
  padding: 12px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.14);
}

.section-compact {
  gap: 9px;
}

.section-title {
  margin: 0;
  color: var(--text-primary);
  font-size: 0.8rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.field {
  display: grid;
  gap: 7px;
}

.field-label {
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
}

input {
  width: 100%;
  height: 36px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-body);
  border-radius: 8px;
  padding: 0 12px;
  font-family: var(--font-mono);
  font-size: 13px;
}

input:focus {
  outline: none;
  border-color: rgba(255, 255, 255, 0.24);
}

.row {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr 1fr;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 4px;
}

.btn-secondary,
.btn-primary {
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  height: 34px;
  padding: 0 14px;
  font-family: var(--font-mono);
  font-size: 12px;
  cursor: pointer;
}

.btn-secondary {
  background: rgba(0, 0, 0, 0.22);
  color: var(--text-body);
}

.btn-primary {
  background: rgba(88, 101, 242, 0.22);
  color: var(--text-primary);
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: default;
}

.error-text {
  margin: 0 2px;
  color: #fda4af;
  font-size: 0.85rem;
  line-height: 1.4;
}

.toggle-row {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  color: var(--text-muted);
  font-size: 0.85rem;
  line-height: 1.35;
}

.toggle-row input {
  width: 18px;
  height: 18px;
  margin-top: 2px;
  accent-color: #0ea5e9;
}

@media (max-width: 640px) {
  .row {
    grid-template-columns: 1fr;
  }

  .actions {
    justify-content: stretch;
  }

  .btn-secondary,
  .btn-primary {
    flex: 1;
  }
}
</style>
