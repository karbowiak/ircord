<script setup lang="ts">
import type { ConnectionInfo, Network } from '../../shared/types/chat'
import { capabilityRows } from '../../shared/utils/connection'

/**
 * The server page's side column: announced and negotiated capabilities, IRCord extensions,
 * and ISUPPORT settings under one text filter. The status filter applies to capabilities.
 */
const props = defineProps<{ network: Network; info?: ConnectionInfo; loading?: boolean }>()
const search = ref('')
const scope = ref<'all' | 'active' | 'offered'>('all')
const needle = computed(() => search.value.trim().toLowerCase())
/** Details from the last connection while disconnected: nothing is active right now. */
const lastSession = computed(() => props.network.status !== 'connected')
const editingAvailable = computed(() => !lastSession.value && props.network.features.edit)
const editingDescription = computed(() => {
  if (lastSession.value) return 'Reconnect to use message editing.'
  return editingAvailable.value
    ? 'Edit your own messages sent from your IRC account.'
    : 'Requires message-tags, echo-message, account-tag, and an allowed ircord/edit tag.'
})
const showEditing = computed(() =>
  `Message editing +ircord/edit ${editingDescription.value}`.toLowerCase().includes(needle.value),
)
const rows = computed(() => (props.info ? capabilityRows(props.info) : []))
const capabilities = computed(() =>
  rows.value.filter(
    (row) =>
      (scope.value === 'all' || row.active === (scope.value === 'active')) &&
      `${row.name} ${row.value} ${row.description}`.toLowerCase().includes(needle.value),
  ),
)
const activeCount = computed(() => rows.value.filter((r) => r.active).length)
const support = computed(() =>
  Object.entries(props.info?.isupport ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .filter(([key, value]) => `${key} ${value}`.toLowerCase().includes(needle.value)),
)
const supportOpen = ref(true)
const extensionsOpen = ref(true)
const capabilitiesOpen = ref(true)
const heading =
  'm-0 flex items-center gap-2 px-1 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground'
</script>
<template>
  <aside
    class="server-capabilities flex min-h-0 min-w-0 flex-col border-l [border-left-style:solid] border-border/45"
    aria-label="Capabilities, IRCord extensions and server settings"
  >
    <div class="flex shrink-0 flex-col gap-2 px-3 pt-3 pb-2">
      <SearchField
        v-model="search"
        placeholder="Filter server details"
        aria-label="Filter server details"
      />
      <AppSegmented
        v-model="scope"
        label="Capability status"
        :options="[
          { value: 'all', label: 'All' },
          { value: 'active', label: lastSession ? 'Last used' : 'Active' },
          { value: 'offered', label: 'Announced' },
        ]"
      />
    </div>
    <div
      class="min-h-0 flex-1 overflow-y-auto px-2 pb-4"
      tabindex="0"
      aria-label="Capabilities, IRCord extensions and server settings"
    >
      <p
        v-if="loading && !info"
        class="m-0 px-1 py-3 text-[length:var(--ui-small)] text-muted-foreground"
        role="status"
      >
        Loading the server handshake…
      </p>
      <template v-else-if="info">
        <h3 :class="[heading, 'h-7']">
          <button
            type="button"
            class="-ml-1 flex items-center gap-1 rounded-sm px-1 hover:text-foreground"
            :aria-expanded="capabilitiesOpen"
            @click="capabilitiesOpen = !capabilitiesOpen"
          >
            <AppIcon
              name="down"
              :size="11"
              class="transition-transform"
              :class="{ '-rotate-90': !capabilitiesOpen }"
            />Capabilities
          </button>
          <span class="font-normal normal-case tracking-normal tabular-nums"
            >{{ activeCount }} / {{ rows.length }}</span
          >
        </h3>
        <template v-if="capabilitiesOpen">
          <p
            v-if="lastSession"
            class="m-0 px-1 pb-2 text-[length:var(--ui-caption)] text-muted-foreground"
          >
            From the last connection; nothing is active while disconnected.
          </p>
          <ul
            v-if="capabilities.length"
            class="m-0 list-none p-0"
          >
            <li
              v-for="row in capabilities"
              :key="row.name"
              class="server-capability rounded-md px-2 py-1.5 hover:bg-accent/50"
            >
              <div class="flex min-w-0 items-center gap-2">
                <i
                  class="size-1.5 shrink-0 rounded-full"
                  :class="row.active && !lastSession ? 'bg-success' : 'bg-border'"
                  aria-hidden="true"
                /><code
                  class="min-w-0 flex-1 truncate font-code text-[length:var(--ui-caption)] text-foreground"
                  :title="row.name"
                  >{{ row.name }}</code
                ><span
                  class="shrink-0 text-[length:var(--ui-label)]"
                  :class="row.active && !lastSession ? 'text-success' : 'text-faint'"
                  >{{ row.active ? (lastSession ? 'Last used' : 'Active') : 'Announced' }}</span
                >
              </div>
              <p
                v-if="row.description"
                class="m-0 mt-0.5 pl-3.5 text-[length:var(--ui-caption)] leading-snug text-muted-foreground"
              >
                {{ row.description }}
              </p>
              <code
                v-if="row.value"
                class="mt-0.5 block pl-3.5 font-code text-[length:var(--ui-label)] wrap-anywhere text-body"
                >{{ row.value }}</code
              >
            </li>
          </ul>
          <p
            v-else
            class="m-0 px-1 py-2 text-[length:var(--ui-small)] text-muted-foreground"
          >
            {{ needle ? 'No capabilities match.' : 'No capabilities in this group.' }}
          </p>
        </template>

        <section aria-label="IRCord extensions">
          <h3 :class="[heading, 'mt-4 h-8']">
            <button
              type="button"
              class="-ml-1 flex items-center gap-1 rounded-sm px-1 hover:text-foreground"
              :aria-expanded="extensionsOpen"
              @click="extensionsOpen = !extensionsOpen"
            >
              <AppIcon
                name="down"
                :size="11"
                class="transition-transform"
                :class="{ '-rotate-90': !extensionsOpen }"
              />IRCord extensions
            </button>
            <span class="font-normal normal-case tracking-normal tabular-nums">{{
              showEditing ? 1 : 0
            }}</span>
          </h3>
          <template v-if="extensionsOpen">
            <p class="m-0 px-1 pb-2 text-[length:var(--ui-caption)] text-muted-foreground">
              Client features that use this connection.
            </p>
            <div
              v-if="showEditing"
              class="server-extension rounded-md px-2 py-1.5 hover:bg-accent/50"
            >
              <div class="flex min-w-0 items-center gap-2">
                <span class="min-w-0 flex-1 text-[length:var(--ui-caption)] text-foreground">
                  Message editing
                </span>
                <span
                  class="shrink-0 text-[length:var(--ui-label)]"
                  :class="editingAvailable ? 'text-success' : 'text-muted-foreground'"
                  >{{ editingAvailable ? 'Available' : 'Unavailable' }}</span
                >
              </div>
              <code class="font-code text-[length:var(--ui-label)] text-muted-foreground">
                +ircord/edit
              </code>
              <p
                class="m-0 mt-0.5 text-[length:var(--ui-caption)] leading-snug text-muted-foreground"
              >
                {{ editingDescription }}
              </p>
            </div>
            <p
              v-else
              class="m-0 px-1 py-2 text-[length:var(--ui-small)] text-muted-foreground"
            >
              No IRCord extensions match.
            </p>
          </template>
        </section>

        <h3 :class="[heading, 'mt-4 h-8']">
          <button
            type="button"
            class="-ml-1 flex items-center gap-1 rounded-sm px-1 hover:text-foreground"
            :aria-expanded="supportOpen"
            @click="supportOpen = !supportOpen"
          >
            <AppIcon
              name="down"
              :size="11"
              class="transition-transform"
              :class="{ '-rotate-90': !supportOpen }"
            />ISUPPORT
          </button>
          <span class="font-normal normal-case tracking-normal tabular-nums">{{
            support.length
          }}</span>
        </h3>
        <template v-if="supportOpen">
          <p class="m-0 px-1 pb-2 text-[length:var(--ui-caption)] text-muted-foreground">
            Limits, channel modes and other settings from the server’s 005 reply.
          </p>
          <dl
            v-if="support.length"
            class="m-0 grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-x-3 px-2 text-[length:var(--ui-caption)] [&_dd]:m-0 [&_dd]:wrap-anywhere [&_dd]:text-body [&_dt]:wrap-anywhere [&_dt]:text-muted-foreground [&>*]:border-b [&>*]:border-border/40 [&>*]:py-1 [&>*]:[border-bottom-style:solid]"
          >
            <template
              v-for="[key, value] in support"
              :key="key"
            >
              <dt class="font-code">{{ key }}</dt>
              <dd>
                <code
                  v-if="value"
                  class="font-code"
                  >{{ value }}</code
                ><span
                  v-else
                  class="text-muted-foreground"
                  >Enabled</span
                >
              </dd>
            </template>
          </dl>
          <p
            v-else
            class="m-0 px-1 py-2 text-[length:var(--ui-small)] text-muted-foreground"
          >
            {{ needle ? 'No server settings match.' : 'No ISUPPORT settings received yet.' }}
          </p>
        </template>
      </template>
      <p
        v-else
        class="m-0 px-1 py-3 text-[length:var(--ui-small)] text-muted-foreground"
      >
        Capabilities appear once IRCord connects.
      </p>
    </div>
  </aside>
</template>
