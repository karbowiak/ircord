<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { channelRights, markdownToIrc, stripFormatting, topPrefix } from '../../shared/utils/irc'
import { listInfo, modeInfo, parseChanmodes, toMask } from '../../shared/utils/channel-modes'
import { channelModeListQuery } from '../queries/chat'

/**
 * Everything IRC lets you see and change about a channel: topic, modes (as advertised by the
 * network's CHANMODES) and its ban/exception/invite/quiet lists. Changes are plain MODE and TOPIC
 * commands; editing is enabled only where your channel rank allows it.
 */
const props = defineProps<{ conversationId: string; initialTab?: string }>()
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const notifications = useNotificationStore()
const { confirm } = usePrompt()
const c = computed(() => chat.data?.conversations.find((x) => x.id === props.conversationId))
const network = computed(() => (c.value ? chat.networkOf(c.value) : undefined))
const name = computed(() => `#${c.value?.name ?? ''}`)
const rights = computed(() => channelRights('you', c.value, network.value))
const groups = computed(() => parseChanmodes(network.value?.isupport))
const myPrefix = computed(() => topPrefix('you', c.value, network.value))
const set = computed(() => new Set(c.value?.modes ?? ''))

const tab = ref(props.initialTab ?? 'overview')
const tabs = computed(() => [
  { id: 'overview', label: 'Overview' },
  { id: 'modes', label: 'Modes' },
  ...[...groups.value.list].map((mode) => ({
    id: `list:${mode}`,
    label: listInfo[mode]?.label ?? `+${mode} list`,
  })),
])

const send = (line: string) => chat.raw(line, network.value?.id)

// ---- Topic ----------------------------------------------------------------------------------
const topic = ref(stripFormatting(c.value?.topic ?? ''))
const topicChanged = computed(() => topic.value !== stripFormatting(c.value?.topic ?? ''))
watch(
  () => c.value?.topic,
  (value) => (topic.value = stripFormatting(value ?? '')),
)
const maxTopic = computed(() => Number(network.value?.isupport.TOPICLEN ?? 390))
function saveTopic() {
  send(`TOPIC ${name.value} :${markdownToIrc(topic.value.trim())}`)
}
const date = (iso?: string) =>
  iso ? new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : ''

// ---- Modes ----------------------------------------------------------------------------------
/** Flags the network advertises, common ones first. */
const flags = computed(() =>
  [...groups.value.flags].sort(
    (a, b) => Number(!!modeInfo[b]) - Number(!!modeInfo[a]) || a.localeCompare(b),
  ),
)
const paramModes = computed(() => [...groups.value.always, ...groups.value.whenSet])
const values = reactive<Record<string, string>>({})
watch(
  () => c.value?.modeParams,
  (params) => {
    for (const mode of paramModes.value) values[mode] = mode === 'k' ? '' : (params?.[mode] ?? '')
  },
  { immediate: true },
)
const toggle = (mode: string) =>
  send(`MODE ${name.value} ${set.value.has(mode) ? '-' : '+'}${mode}`)
/** Number inputs give numbers through v-model; compare everything as text. */
const valueOf = (mode: string) => String(values[mode] ?? '').trim()
function setParam(mode: string) {
  const value = valueOf(mode)
  if (value) send(`MODE ${name.value} +${mode} ${value}`)
}
function clearParam(mode: string) {
  // Removing a key needs the key on some servers; send the current one when known.
  const current = groups.value.always.includes(mode) ? ` ${c.value?.modeParams?.[mode] ?? '*'}` : ''
  send(`MODE ${name.value} -${mode}${current}`)
}

// ---- Lists ----------------------------------------------------------------------------------
const listMode = computed(() => (tab.value.startsWith('list:') ? tab.value.slice(5) : ''))
const list = useQuery(() => ({
  ...channelModeListQuery({ conversationId: props.conversationId, mode: listMode.value || 'b' }),
  enabled: !!listMode.value,
}))
const newMask = ref('')
let refresh: ReturnType<typeof setTimeout> | undefined
/** List changes arrive as mode events; refetch shortly after sending one. */
function refetchSoon() {
  clearTimeout(refresh)
  refresh = setTimeout(() => void list.refetch(), 400)
}
function addMask() {
  if (!newMask.value.trim()) return
  send(`MODE ${name.value} +${listMode.value} ${toMask(newMask.value)}`)
  newMask.value = ''
  refetchSoon()
}
async function removeMask(mask: string) {
  if (
    await confirm({
      title: `Remove ${listInfo[listMode.value]?.singular ?? 'entry'}?`,
      body: mask,
      confirm: 'Remove',
    })
  ) {
    send(`MODE ${name.value} -${listMode.value} ${mask}`)
    refetchSoon()
  }
}
onBeforeUnmount(() => clearTimeout(refresh))

async function leave() {
  if (!c.value) return
  if (await confirm({ title: `Leave ${name.value}?`, confirm: 'Leave', danger: true })) {
    chat.leave.mutate({ c: c.value })
    emit('close')
  }
}
const rankPhrase: Record<string, string> = { '~': 'the owner', '&': 'an admin', '@': 'an operator' }
const rankName: Record<string, string> = {
  '~': 'Owner',
  '&': 'Admin',
  '@': 'Operator',
  '%': 'Half-operator',
  '+': 'Voiced',
}
</script>
<template>
  <AppDialog
    :title="`${name} settings`"
    panel
    @close="emit('close')"
  >
    <div
      v-if="c"
      class="settings-layout channel-settings"
    >
      <nav
        class="settings-nav"
        aria-label="Channel settings"
      >
        <button
          v-for="item in tabs"
          :key="item.id"
          :class="{ active: tab === item.id }"
          :aria-current="tab === item.id ? 'page' : undefined"
          @click="tab = item.id"
        >
          {{ item.label }}
        </button>
      </nav>
      <div class="settings-content">
        <p
          class="rank-note"
          :class="{ 'can-edit': rights.op }"
        >
          <AppIcon
            :name="rights.op ? 'shield' : 'lock'"
            :size="15"
          />
          <span v-if="rights.op"
            >You’re {{ rankPhrase[myPrefix] ?? 'an operator' }} here and can change everything
            below.</span
          >
          <span v-else-if="rights.kick"
            >As a half-operator you can manage lists and the topic, but not modes.</span
          >
          <span v-else>Read-only: changing these needs operator status (@) in {{ name }}.</span>
        </p>

        <!-- Overview -->
        <div
          v-if="tab === 'overview'"
          class="appearance-settings"
        >
          <section class="setting-group">
            <h3>Topic</h3>
            <form
              class="topic-editor"
              @submit.prevent="saveTopic"
            >
              <textarea
                v-model="topic"
                class="text-input"
                rows="3"
                :maxlength="maxTopic"
                :readonly="!rights.topic"
                aria-label="Channel topic"
                placeholder="No topic set"
              />
              <div class="topic-meta">
                <span v-if="c.topicSetBy"
                  >Set by <strong>{{ c.topicSetBy }}</strong>
                  <template v-if="c.topicSetAt"> · {{ date(c.topicSetAt) }}</template></span
                >
                <span
                  v-if="rights.topic"
                  class="topic-count"
                  >{{ topic.length }}/{{ maxTopic }}</span
                >
                <button
                  v-if="rights.topic"
                  class="button primary small"
                  :disabled="!topicChanged"
                >
                  Save topic
                </button>
              </div>
              <p
                v-if="!rights.topic && set.has('t')"
                class="field-hint"
              >
                The topic is locked (+t): only operators can change it.
              </p>
              <p
                v-else-if="rights.topic"
                class="field-hint"
              >
                **bold**, *italic* and __underline__ are sent as IRC formatting.
              </p>
            </form>
          </section>
          <section class="setting-group">
            <h3>About</h3>
            <dl class="channel-facts">
              <div>
                <dt>Network</dt>
                <dd>{{ network?.name }}</dd>
              </div>
              <div>
                <dt>People</dt>
                <dd>{{ c.memberIds.length }}</dd>
              </div>
              <div>
                <dt>Modes</dt>
                <dd>
                  <code>+{{ c.modes || '' }}</code>
                </dd>
              </div>
              <div>
                <dt>Your rank</dt>
                <dd>{{ rankName[myPrefix] ?? 'Member' }}</dd>
              </div>
              <div v-if="c.createdAt">
                <dt>Created</dt>
                <dd>{{ date(c.createdAt) }}</dd>
              </div>
              <div v-if="c.url">
                <dt>Website</dt>
                <dd>
                  <a
                    :href="c.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    >{{ c.url }}</a
                  >
                </dd>
              </div>
            </dl>
          </section>
          <section class="setting-group">
            <h3>You</h3>
            <div class="setting-row">
              <span class="setting-copy"
                ><strong>Notifications</strong
                ><small>Also available from the channel’s right-click menu.</small></span
              >
              <div
                class="segmented"
                role="group"
                aria-label="Notifications for this channel"
              >
                <button
                  v-for="option in [
                    { value: null, label: 'Default' },
                    { value: 'all', label: 'All' },
                    { value: 'mentions', label: 'Mentions' },
                    { value: 'none', label: 'Mute' },
                  ] as const"
                  :key="String(option.value)"
                  :class="{
                    active: (notifications.settings.levels[c.id] ?? null) === option.value,
                  }"
                  :aria-pressed="(notifications.settings.levels[c.id] ?? null) === option.value"
                  @click="notifications.setLevel(c, option.value)"
                >
                  {{ option.label }}
                </button>
              </div>
            </div>
            <div
              v-if="chat.data?.capabilities?.serverPreviews"
              class="setting-row"
            >
              <span class="setting-copy"
                ><strong>Server link previews</strong
                ><small
                  >IRCord’s server reads the page for links here (title, summary, and the GIF for
                  Tenor and GIPHY links), so this device doesn’t open the page. Preview images and
                  GIFs still load from the site’s media host unless image previews are off. Only
                  affects you.</small
                ></span
              >
              <button
                role="switch"
                class="toggle"
                :class="{ on: c.previewMode === 'server' }"
                :aria-checked="c.previewMode === 'server'"
                aria-label="Server link previews"
                @click="
                  chat.previewMode.mutate({
                    c,
                    mode: c.previewMode === 'server' ? 'client' : 'server',
                  })
                "
              >
                <span />
              </button>
            </div>
            <div class="setting-row">
              <span class="setting-copy"
                ><strong>Leave channel</strong><small>You can join again any time.</small></span
              >
              <button
                class="button small danger"
                @click="leave"
              >
                Leave
              </button>
            </div>
          </section>
        </div>

        <!-- Modes -->
        <div
          v-else-if="tab === 'modes'"
          class="appearance-settings"
        >
          <section class="setting-group">
            <h3>Access and behaviour</h3>
            <div
              v-for="mode in flags"
              :key="mode"
              class="setting-row"
            >
              <span class="setting-copy"
                ><strong
                  ><code class="mode-letter">+{{ mode }}</code>
                  {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
                ><small
                  >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.'
                  }}<template v-if="modeInfo[mode]?.varies">
                    Its exact meaning depends on the server.</template
                  ></small
                ></span
              >
              <button
                role="switch"
                class="toggle"
                :class="{ on: set.has(mode) }"
                :aria-checked="set.has(mode)"
                :aria-label="`+${mode} ${modeInfo[mode]?.label ?? ''}`"
                :disabled="!rights.op"
                @click="toggle(mode)"
              >
                <span />
              </button>
            </div>
          </section>
          <section
            v-if="paramModes.length"
            class="setting-group"
          >
            <h3>Settings with a value</h3>
            <form
              v-for="mode in paramModes"
              :key="mode"
              class="setting-row param-mode"
              @submit.prevent="setParam(mode)"
            >
              <span class="setting-copy"
                ><strong
                  ><code class="mode-letter">+{{ mode }}</code>
                  {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
                ><small
                  >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.' }}
                  <template v-if="set.has(mode)"
                    >Currently <strong>{{ mode === 'k' ? 'set' : c.modeParams?.[mode] }}</strong
                    >.</template
                  ></small
                ></span
              >
              <div class="param-controls">
                <input
                  v-model="values[mode]"
                  class="text-input"
                  :type="mode === 'l' ? 'number' : mode === 'k' ? 'password' : 'text'"
                  :min="mode === 'l' ? 1 : undefined"
                  :placeholder="
                    mode === 'k'
                      ? set.has('k')
                        ? 'New key'
                        : 'Key'
                      : mode === 'l'
                        ? '50'
                        : 'Value'
                  "
                  :aria-label="`Value for +${mode}`"
                  :disabled="!rights.op"
                  autocomplete="off"
                /><button
                  class="button secondary small"
                  :disabled="!rights.op || !valueOf(mode)"
                >
                  Set</button
                ><button
                  v-if="set.has(mode)"
                  type="button"
                  class="button secondary small"
                  :disabled="!rights.op"
                  @click="clearParam(mode)"
                >
                  Remove
                </button>
              </div>
            </form>
          </section>
          <p class="field-hint">
            Registration, founders and access lists are handled by the network’s services (for
            example ChanServ). Use <code>/msg ChanServ help</code> to see what they offer.
          </p>
        </div>

        <!-- List modes -->
        <div
          v-else
          class="appearance-settings"
        >
          <section class="setting-group">
            <h3>{{ listInfo[listMode]?.label ?? `+${listMode} list` }}</h3>
            <p class="field-hint">
              {{ listInfo[listMode]?.description ?? 'A list kept by the server.' }} Masks look like
              <code>nick!user@host</code>; <code>*</code> matches anything.
            </p>
            <form
              v-if="rights.ban"
              class="mask-add"
              @submit.prevent="addMask"
            >
              <input
                v-model="newMask"
                class="text-input"
                placeholder="nick, or nick!user@host"
                :aria-label="`Add ${listInfo[listMode]?.singular ?? 'entry'}`"
                autocomplete="off"
              /><button
                class="button primary small"
                :disabled="!newMask.trim()"
              >
                Add {{ listInfo[listMode]?.singular ?? 'entry' }}
              </button>
            </form>
            <p
              v-if="list.isLoading.value && !list.data.value"
              class="field-hint"
            >
              Loading…
            </p>
            <p
              v-else-if="list.error.value"
              class="form-error"
            >
              {{ errorText(list.error.value) }}
            </p>
            <p
              v-else-if="!list.data.value?.items.length"
              class="list-empty"
            >
              Nothing here.
            </p>
            <ul
              v-else
              class="mask-list"
            >
              <li
                v-for="entry in list.data.value?.items"
                :key="entry.mask"
              >
                <code>{{ entry.mask }}</code>
                <span class="mask-meta"
                  ><template v-if="entry.setBy">by {{ entry.setBy }}</template
                  ><template v-if="entry.setAt"> · {{ date(entry.setAt) }}</template></span
                >
                <button
                  v-if="rights.ban"
                  class="icon-button"
                  :aria-label="`Remove ${entry.mask}`"
                  @click="removeMask(entry.mask)"
                >
                  <AppIcon
                    name="trash"
                    :size="15"
                  />
                </button>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </div>
  </AppDialog>
</template>
