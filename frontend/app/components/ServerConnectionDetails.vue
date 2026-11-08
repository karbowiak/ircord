<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { Network } from '../../shared/types/chat'
import { capabilityDescriptions, capabilityRows } from '../../shared/utils/connection'
import { connectionQuery } from '../queries/chat'

const props = defineProps<{ network: Network }>()
const connection = useQuery(() => connectionQuery(props.network.id))
const info = computed(() => connection.data.value)
const expanded = ref(false)
const view = ref<'capabilities' | 'isupport' | 'connection'>('capabilities')
const scope = ref<'all' | 'active' | 'offered'>('all')
const search = ref('')
const panelId = useId()
const active = computed(() =>
  props.network.status === 'connected'
    ? [...(info.value?.negotiatedCapabilities ?? props.network.capabilities)].sort()
    : [],
)
const rows = computed(() => (info.value ? capabilityRows(info.value) : []))
const filtered = computed(() =>
  rows.value.filter(
    (row) =>
      (scope.value === 'all' || row.active === (scope.value === 'active')) &&
      `${row.name} ${row.value} ${row.description}`
        .toLowerCase()
        .includes(search.value.trim().toLowerCase()),
  ),
)
const support = computed(() =>
  Object.entries(info.value?.isupport ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .filter(([key, value]) =>
      `${key} ${value}`.toLowerCase().includes(search.value.trim().toLowerCase()),
    ),
)
const announced = computed(() => Object.keys(info.value?.offeredCapabilities ?? {}).length)
const state = computed(
  () =>
    ({
      connected: 'Connected',
      connecting: 'Connecting',
      disconnected: 'Disconnected',
      error: 'Connection failed',
    })[props.network.status],
)
const lastSession = computed(() => props.network.status !== 'connected')
const timestamp = (value: string) =>
  new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
watch(view, () => {
  search.value = ''
})
</script>

<template>
  <section
    class="server-connection"
    :class="{ 'is-expanded': expanded }"
    aria-label="Server connection"
  >
    <button
      class="server-connection-toggle"
      :aria-expanded="expanded"
      :aria-controls="panelId"
      :aria-label="`Server capabilities and connection details (${state})`"
      @click="expanded = !expanded"
    >
      <span class="server-connection-identity">
        <span
          class="connection-state-dot"
          :class="network.status"
          :title="state"
        />
        <strong :title="info?.serverName || network.host">{{
          info?.serverName || network.host
        }}</strong>
        <span class="connection-state-label">{{ state }}</span>
      </span>
      <span class="server-capability-count">
        {{ active.length }} active<span
          v-if="info"
          class="server-announced-count"
        >
          / {{ announced }} announced</span
        >
      </span>
      <AppIcon
        :name="expanded ? 'down' : 'right'"
        :size="16"
      />
    </button>

    <div
      v-if="active.length"
      class="server-capability-strip"
      aria-label="Active capabilities"
      tabindex="0"
    >
      <span
        v-for="capability in active"
        :key="capability"
        class="server-capability-chip"
        :title="capabilityDescriptions[capability] || capability"
        >{{ capability }}</span
      >
    </div>
    <p
      v-else
      class="server-connection-empty"
    >
      {{
        network.status === 'connected'
          ? 'No capabilities negotiated.'
          : 'Capabilities become active when this network connects.'
      }}
    </p>

    <div
      v-if="expanded"
      :id="panelId"
      class="server-connection-details"
      tabindex="0"
      aria-label="Server handshake details"
    >
      <nav
        class="server-detail-tabs"
        aria-label="Connection information"
      >
        <button
          :aria-pressed="view === 'capabilities'"
          @click="view = 'capabilities'"
        >
          Capabilities
        </button>
        <button
          :aria-pressed="view === 'isupport'"
          @click="view = 'isupport'"
        >
          ISUPPORT
        </button>
        <button
          :aria-pressed="view === 'connection'"
          @click="view = 'connection'"
        >
          Connection
        </button>
      </nav>

      <div
        v-if="connection.isLoading.value && !info"
        class="server-detail-status"
        role="status"
      >
        Loading the server handshake…
      </div>
      <div
        v-else-if="connection.error.value"
        class="server-detail-status"
        role="status"
      >
        <span>Couldn’t load the connection details.</span>
        <button
          class="text-button"
          @click="connection.refetch()"
        >
          Try again
        </button>
      </div>
      <template v-else-if="info">
        <p
          v-if="lastSession"
          class="server-detail-note"
        >
          Details from the last connection. Capabilities are currently inactive.
        </p>
        <div
          v-if="view !== 'connection'"
          class="server-detail-controls"
        >
          <label class="server-detail-search">
            <AppIcon
              name="search"
              :size="14"
            />
            <input
              v-model="search"
              :aria-label="view === 'capabilities' ? 'Filter capabilities' : 'Filter ISUPPORT'"
              :placeholder="
                view === 'capabilities' ? 'Find a capability…' : 'Find a server setting…'
              "
              type="search"
            />
          </label>
          <select
            v-if="view === 'capabilities'"
            v-model="scope"
            class="server-capability-scope"
            aria-label="Capability status"
          >
            <option value="all">All capabilities</option>
            <option value="active">{{ lastSession ? 'Last negotiated' : 'Active' }}</option>
            <option value="offered">Announced only</option>
          </select>
        </div>

        <div
          class="server-detail-scroll"
          :aria-label="
            view === 'capabilities'
              ? 'Capability details'
              : view === 'isupport'
                ? 'ISUPPORT details'
                : 'Connection details'
          "
        >
          <template v-if="view === 'capabilities'">
            <p class="server-detail-note">
              Announced by the server; active capabilities were negotiated by this connection.
              Individual actions can also require permission.
            </p>
            <ul
              v-if="filtered.length"
              class="server-capability-list"
            >
              <li
                v-for="row in filtered"
                :key="row.name"
              >
                <div class="server-capability-name">
                  <code>{{ row.name }}</code
                  ><span
                    class="server-capability-status"
                    :class="{ active: row.active && !lastSession }"
                    >{{
                      row.active ? (lastSession ? 'Last negotiated' : 'Active') : 'Announced'
                    }}</span
                  >
                </div>
                <p v-if="row.description">{{ row.description }}</p>
                <code
                  v-if="row.value"
                  class="server-capability-value"
                  >{{ row.value }}</code
                >
              </li>
            </ul>
            <p
              v-else
              class="server-detail-empty"
            >
              {{ search ? 'No capabilities match your search.' : 'No capabilities in this group.' }}
            </p>
          </template>
          <template v-else-if="view === 'isupport'">
            <p class="server-detail-note">
              Limits, channel modes and other settings announced in the server’s 005 reply.
            </p>
            <dl
              v-if="support.length"
              class="server-detail-table"
            >
              <div
                v-for="[key, value] in support"
                :key="key"
              >
                <dt>
                  <code>{{ key }}</code>
                </dt>
                <dd>
                  <code v-if="value">{{ value }}</code
                  ><span v-else>Enabled</span>
                </dd>
              </div>
            </dl>
            <p
              v-else
              class="server-detail-empty"
            >
              {{
                search
                  ? 'No server settings match your search.'
                  : 'No ISUPPORT settings received yet.'
              }}
            </p>
          </template>
          <dl
            v-else
            class="server-detail-table"
          >
            <div>
              <dt>Address</dt>
              <dd>
                <code>{{ network.host }}:{{ network.port }}</code>
              </dd>
            </div>
            <div>
              <dt>Server</dt>
              <dd>{{ info.serverName || 'Not received yet' }}</dd>
            </div>
            <div>
              <dt>Status</dt>
              <dd>{{ state }}</dd>
            </div>
            <div v-if="info.connectedAt">
              <dt>Connected</dt>
              <dd>{{ timestamp(info.connectedAt) }}</dd>
            </div>
            <div>
              <dt>Encryption</dt>
              <dd>
                {{
                  info.tls
                    ? `${info.tls.version} · ${info.tls.verified ? 'Certificate verified' : 'Certificate not verified'}`
                    : network.status === 'connected'
                      ? 'Unencrypted IRC connection'
                      : 'Not established'
                }}
              </dd>
            </div>
            <div v-if="info.tls?.cipher">
              <dt>Cipher</dt>
              <dd>
                <code>{{ info.tls.cipher }}</code>
              </dd>
            </div>
            <div v-if="info.tls?.certificateSha256">
              <dt>Certificate SHA-256</dt>
              <dd>
                <code>{{ info.tls.certificateSha256 }}</code>
              </dd>
            </div>
            <div v-if="info.upstreamFilehostUrl">
              <dt>Server filehost</dt>
              <dd>{{ info.upstreamFilehostUrl }}</dd>
            </div>
            <div v-if="info.lastError">
              <dt>Last server error</dt>
              <dd>{{ info.lastError }}</dd>
            </div>
          </dl>
        </div>
      </template>
    </div>
  </section>
</template>
