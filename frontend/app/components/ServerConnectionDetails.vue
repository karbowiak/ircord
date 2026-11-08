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
    class="server-connection [flex:0_0_auto] min-w-0 min-h-0 flex flex-col border-b [border-bottom-style:solid] border-b-border bg-sidebar"
    :class="{ 'is-expanded': expanded }"
    aria-label="Server connection"
  >
    <button
      class="server-connection-toggle [flex:0_0_auto] flex items-center gap-2.5 w-full min-h-11 py-2 px-4 text-left"
      :aria-expanded="expanded"
      :aria-controls="panelId"
      :aria-label="`Server capabilities and connection details (${state})`"
      @click="expanded = !expanded"
    >
      <span class="server-connection-identity flex items-center gap-2 min-w-0 flex-1">
        <span
          class="connection-state-dot w-1.5 h-1.5 [flex:0_0_6px] rounded-full bg-muted-foreground"
          :class="network.status"
          :title="state"
        />
        <strong :title="info?.serverName || network.host">{{
          info?.serverName || network.host
        }}</strong>
        <span class="connection-state-label text-[12px] text-muted-foreground whitespace-nowrap">{{
          state
        }}</span>
      </span>
      <span
        class="server-capability-count text-[12px] text-muted-foreground whitespace-nowrap ml-auto"
      >
        {{ active.length }} active<span
          v-if="info"
          class="server-announced-count text-muted-foreground"
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
      class="server-capability-strip [flex:0_0_auto] flex gap-[5px] pt-0 pr-4 pb-2.5 pl-4 overflow-x-auto [scrollbar-width:thin]"
      aria-label="Active capabilities"
      tabindex="0"
    >
      <span
        v-for="capability in active"
        :key="capability"
        class="server-capability-chip [flex:0_0_auto] py-[3px] px-1.5 rounded-sm bg-secondary text-body font-code text-[11px] leading-[1.4]"
        :title="capabilityDescriptions[capability] || capability"
        >{{ capability }}</span
      >
    </div>
    <p
      v-else
      class="server-connection-empty pt-0 pr-4 pb-2.5 pl-4 m-0 text-[12px] text-muted-foreground"
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
      class="server-connection-details min-h-0 [flex:1_1_auto] overflow-y-auto overscroll-contain border-t [border-top-style:solid] border-t-border"
      tabindex="0"
      aria-label="Server handshake details"
    >
      <UiTabs v-model="view">
        <UiTabsList
          class="server-detail-tabs sticky top-0 z-1 bg-sidebar gap-1 pt-1.5 pr-3 pb-0 pl-3"
          aria-label="Connection information"
        >
          <UiTabsTrigger
            variant="pill"
            value="capabilities"
          >
            Capabilities
          </UiTabsTrigger>
          <UiTabsTrigger
            variant="pill"
            value="isupport"
          >
            ISUPPORT
          </UiTabsTrigger>
          <UiTabsTrigger
            variant="pill"
            value="connection"
          >
            Connection
          </UiTabsTrigger>
        </UiTabsList>
        <UiTabsContent :value="view">
          <div
            v-if="connection.isLoading.value && !info"
            class="server-detail-status flex flex-wrap gap-2 items-center p-4 text-[13px] text-muted-foreground"
            role="status"
          >
            Loading the server handshake…
          </div>
          <div
            v-else-if="connection.error.value"
            class="server-detail-status flex flex-wrap gap-2 items-center p-4 text-[13px] text-muted-foreground"
            role="status"
          >
            <span>Couldn’t load the connection details.</span>
            <UiButton
              variant="link"
              size="text"
              @click="connection.refetch()"
            >
              Try again
            </UiButton>
          </div>
          <template v-else-if="info">
            <p
              v-if="lastSession"
              class="server-detail-note max-w-[75ch] text-muted-foreground text-[12px] leading-[1.5] mt-0 mr-0 mb-3 ml-0"
            >
              Details from the last connection. Capabilities are currently inactive.
            </p>
            <div
              v-if="view !== 'connection'"
              class="server-detail-controls flex gap-2.5 items-center pt-3 pr-4 pb-0 pl-4"
            >
              <SearchField
                v-model="search"
                class="server-detail-search h-auto min-h-8.5 max-w-85 flex-1 gap-2 px-2.5 text-muted-foreground"
                :aria-label="view === 'capabilities' ? 'Filter capabilities' : 'Filter ISUPPORT'"
                :placeholder="
                  view === 'capabilities' ? 'Find a capability…' : 'Find a server setting…'
                "
                type="search"
              />
              <AppSelect
                v-if="view === 'capabilities'"
                v-model="scope"
                :options="[
                  { value: 'all', label: 'All capabilities' },
                  { value: 'active', label: lastSession ? 'Last negotiated' : 'Active' },
                  { value: 'offered', label: 'Announced only' },
                ]"
                class="server-capability-scope [flex:0_0_auto] w-auto max-w-1/2 min-w-0 min-h-8.5 py-0 px-2 text-body bg-rail border border-solid border-border rounded-sm text-[12px]"
                aria-label="Capability status"
              />
            </div>

            <div
              class="server-detail-scroll pt-3 pr-4 pb-4 pl-4"
              :aria-label="
                view === 'capabilities'
                  ? 'Capability details'
                  : view === 'isupport'
                    ? 'ISUPPORT details'
                    : 'Connection details'
              "
            >
              <template v-if="view === 'capabilities'">
                <p
                  class="server-detail-note max-w-[75ch] text-muted-foreground text-[12px] leading-[1.5] mt-0 mr-0 mb-3 ml-0"
                >
                  Announced by the server; active capabilities were negotiated by this connection.
                  Individual actions can also require permission.
                </p>
                <ul
                  v-if="filtered.length"
                  class="server-capability-list grid [grid-template-columns:repeat(auto-fit,_minmax(min(250px,_100%),_1fr))] gap-x-6 m-0 p-0 list-none"
                >
                  <li
                    v-for="row in filtered"
                    :key="row.name"
                  >
                    <div
                      class="server-capability-name flex flex-wrap gap-[6px_10px] items-baseline"
                    >
                      <code>{{ row.name }}</code
                      ><span
                        class="server-capability-status ml-auto text-[11px] text-muted-foreground whitespace-nowrap"
                        :class="{ active: row.active && !lastSession }"
                        >{{
                          row.active ? (lastSession ? 'Last negotiated' : 'Active') : 'Announced'
                        }}</span
                      >
                    </div>
                    <p v-if="row.description">{{ row.description }}</p>
                    <code
                      v-if="row.value"
                      class="server-capability-value font-code block mt-[5px] text-[11px] text-body wrap-anywhere"
                      >{{ row.value }}</code
                    >
                  </li>
                </ul>
                <p
                  v-else
                  class="server-detail-empty my-3 mx-0 text-[13px] text-muted-foreground"
                >
                  {{
                    search ? 'No capabilities match your search.' : 'No capabilities in this group.'
                  }}
                </p>
              </template>
              <template v-else-if="view === 'isupport'">
                <p
                  class="server-detail-note max-w-[75ch] text-muted-foreground text-[12px] leading-[1.5] mt-0 mr-0 mb-3 ml-0"
                >
                  Limits, channel modes and other settings announced in the server’s 005 reply.
                </p>
                <dl
                  v-if="support.length"
                  class="server-detail-table m-0"
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
                  class="server-detail-empty my-3 mx-0 text-[13px] text-muted-foreground"
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
                class="server-detail-table m-0"
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
        </UiTabsContent>
      </UiTabs>
    </div>
  </section>
</template>
