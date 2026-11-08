<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { channelRights, markdownToIrc, stripFormatting, topPrefix } from '../../shared/utils/irc'
import {
  accountExtban,
  listInfo,
  maskMatches,
  modeInfo,
  muteExtban,
  parseChanmodes,
  parseExtbans,
  previewMask,
} from '../../shared/utils/channel-modes'
import { channelModeListQuery } from '../queries/chat'
import type { ChannelSettingsChange } from '../../shared/types/chat'

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
  { id: 'registration', label: 'Registration' },
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
const extbans = computed(() => parseExtbans(network.value?.isupport))
/** What Add will send, shown before it's sent. */
const preview = computed(() => previewMask(newMask.value, extbans.value))
const accountPrefix = computed(() => accountExtban(extbans.value))
const mutePrefix = computed(() => muteExtban(extbans.value))
/**
 * Who in the channel the mask would match right now. People whose username and host aren't
 * known yet only count when the mask doesn't look at them.
 */
const matchedHere = computed(() => {
  const mask = preview.value.mask
  if (!mask || preview.value.extban || !c.value) return []
  const [nickPart = '*', rest = '*@*'] = mask.split('!')
  const loose = /^[*?]*@[*?]*$/.test(rest)
  return c.value.memberIds.flatMap((id) => {
    const m = chat.member(id)
    const nick = id === 'you' ? (network.value?.nick ?? m?.nick) : m?.nick
    if (!nick) return []
    const hit =
      m?.username && m.host
        ? maskMatches(mask, `${nick}!${m.username}@${m.host}`)
        : loose && maskMatches(nickPart, nick)
    return hit ? [id === 'you' ? `${nick} (you)` : nick] : []
  })
})
const maskInput = ref<HTMLInputElement | null>(null)
/** Examples only fill the field; nothing is sent until Add. */
function useExample(text: string) {
  newMask.value = text
  maskInput.value?.focus()
}
const examples = computed(() => [
  {
    text: 'alice',
    explain:
      'Shorthand for alice!*@*: whoever uses the nick alice, now or later. Changing nick gets around it.',
  },
  { text: '*!*@203.0.113.7', explain: 'Anyone connecting from the address 203.0.113.7.' },
  {
    text: '*!*@*.example.net',
    explain:
      'Anyone whose host ends in .example.net. On an internet provider’s hosts that can be a lot of people.',
  },
  {
    text: '*!*alice@*',
    explain:
      'Any username ending in alice: ~alice and alice, but also malice. From any host, with any nick.',
  },
  ...(accountPrefix.value
    ? [
        {
          text: `${accountPrefix.value}alice`,
          explain: 'Whoever is logged in to the account alice, whatever nick or host they use.',
        },
      ]
    : []),
  ...(mutePrefix.value && listMode.value === 'b'
    ? [
        {
          text: 'm:alice',
          explain: 'Mutes alice!*@* instead of banning: they can stay but can’t talk.',
        },
      ]
    : []),
])
function addMask() {
  const { mask, error } = preview.value
  if (!mask || error) return
  // Not sent (disconnected): keep what was typed so it can be added once reconnected.
  if (send(`MODE ${name.value} +${listMode.value} ${mask}`) === false) return
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
// ---- Channel-wide settings -----------------------------------------------------------------
const shared = computed(() => c.value?.channelSettings)
const canShare = computed(() => !!shared.value && rights.value.op)
const levelName: Record<string, string> = { all: 'All', mentions: 'Mentions', none: 'Mute' }
function changeShared(changes: ChannelSettingsChange) {
  const current = shared.value
  if (!c.value || !current || !canShare.value) return
  // Choosing the value that is already set changes nothing; don't send it.
  if ((changes.notifications ?? current.notifications) === current.notifications) return
  return chat.channelSettings.mutateAsync({ c: c.value, changes })
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
      class="settings-layout flex-1 min-h-0 grid [grid-template-columns:200px_minmax(0,_1fr)] [grid-template-rows:auto_minmax(0,_1fr)] [grid-template-areas:'search_content'_'nav_content'] channel-settings"
    >
      <nav
        class="settings-nav [grid-area:nav] flex flex-col gap-0.5 py-3 px-2 bg-sidebar overflow-y-auto"
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
      <div class="settings-content pt-5 pr-7 pb-7 pl-7 overflow-y-auto">
        <!-- ChanServ permissions follow accounts, not channel rank. -->
        <p
          v-if="tab !== 'registration'"
          class="rank-note flex items-center gap-2 mb-4.5 py-2.5 px-3 rounded-md bg-card text-[13px] text-muted-foreground"
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
          class="appearance-settings grid gap-6"
        >
          <section class="setting-group grid gap-3.5">
            <h3>Topic</h3>
            <form
              class="topic-editor grid gap-2"
              @submit.prevent="saveTopic"
            >
              <UiTextarea
                v-model="topic"
                rows="3"
                :maxlength="maxTopic"
                :readonly="!rights.topic"
                aria-label="Channel topic"
                placeholder="No topic set"
              />
              <div class="topic-meta flex items-center gap-3 text-[12px] text-faint">
                <span v-if="c.topicSetBy"
                  >Set by <strong>{{ c.topicSetBy }}</strong>
                  <template v-if="c.topicSetAt"> · {{ date(c.topicSetAt) }}</template></span
                >
                <span
                  v-if="rights.topic"
                  class="topic-count tabular-nums"
                  >{{ topic.length }}/{{ maxTopic }}</span
                >
                <UiButton
                  size="sm"
                  v-if="rights.topic"
                  :disabled="!topicChanged"
                >
                  Save topic
                </UiButton>
              </div>
              <p
                v-if="!rights.topic && set.has('t')"
                class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >
                The topic is locked (+t): only operators can change it.
              </p>
              <p
                v-else-if="rights.topic"
                class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >
                **bold**, *italic* and __underline__ are sent as IRC formatting.
              </p>
            </form>
          </section>
          <section class="setting-group grid gap-3.5">
            <h3>About</h3>
            <dl
              class="channel-facts grid [grid-template-columns:repeat(auto-fill,_minmax(180px,_1fr))] gap-[12px_20px] m-0"
            >
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
          <section
            class="setting-group grid gap-3.5"
            aria-labelledby="channel-wide-heading"
          >
            <h3 id="channel-wide-heading">For everyone in {{ name }}</h3>
            <p
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5 channel-wide-note"
            >
              <template v-if="!shared"
                >These channel-wide settings show once you’re in the channel and
                connected.</template
              >
              <template v-else-if="rights.op"
                >These apply to every IRCord member of this channel. Operators (@) and higher can
                change them.</template
              >
              <template v-else
                >Set by the channel’s operators for everyone here. Changing them needs operator
                status (@).</template
              >
              <template v-if="shared?.updatedBy">
                Last changed by <strong>{{ shared.updatedBy }}</strong
                ><template v-if="shared.updatedAt"> · {{ date(shared.updatedAt) }}</template
                >.</template
              >
            </p>
            <div class="setting-row flex items-center justify-between gap-4">
              <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
                ><strong>Notifications</strong
                ><small
                  >Default uses each member’s own channel default (yours:
                  {{ levelName[notifications.settings.channelDefault] }}). Mute silences the
                  channel, mentions included.</small
                ></span
              >
              <AppSegmented
                :model-value="shared?.notifications ?? 'default'"
                label="Notifications for everyone in this channel"
                :options="
                  [
                    { value: 'default', label: 'Default' },
                    { value: 'all', label: 'All' },
                    { value: 'mentions', label: 'Mentions' },
                    { value: 'none', label: 'Mute' },
                  ] as const
                "
                :disabled="!canShare"
                reselect
                @change="changeShared({ notifications: $event })"
              />
            </div>
          </section>
          <section class="setting-group grid gap-3.5">
            <h3>You</h3>
            <div class="setting-row flex items-center justify-between gap-4">
              <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
                ><strong>Your notifications</strong
                ><small
                  >This device’s notification permission, quiet hours, highlight words, the ignore
                  list and your own channel default stay personal. Find them in User settings →
                  Notifications.</small
                ></span
              >
            </div>
            <div class="setting-row flex items-center justify-between gap-4">
              <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
                ><strong>Leave channel</strong><small>You can join again any time.</small></span
              >
              <UiButton
                variant="destructive"
                size="sm"
                @click="leave"
              >
                Leave
              </UiButton>
            </div>
          </section>
        </div>

        <!-- Modes -->
        <div
          v-else-if="tab === 'modes'"
          class="appearance-settings grid gap-6"
        >
          <section class="setting-group grid gap-3.5">
            <h3>Access and behaviour</h3>
            <div
              v-for="mode in flags"
              :key="mode"
              class="setting-row flex items-center justify-between gap-4"
            >
              <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
                ><strong
                  ><code
                    class="mode-letter font-code text-[12px] font-[600] py-[1px] px-[5px] mr-1 rounded-[4px] bg-rail text-primary"
                    >+{{ mode }}</code
                  >
                  {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
                ><small
                  >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.'
                  }}<template v-if="modeInfo[mode]?.varies">
                    Its exact meaning depends on the server.</template
                  ></small
                ></span
              >
              <AppSwitch
                :model-value="set.has(mode)"
                :aria-label="`+${mode} ${modeInfo[mode]?.label ?? ''}`"
                :disabled="!rights.op"
                @update:model-value="toggle(mode)"
              />
            </div>
          </section>
          <section
            v-if="paramModes.length"
            class="setting-group grid gap-3.5"
          >
            <h3>Settings with a value</h3>
            <form
              v-for="mode in paramModes"
              :key="mode"
              class="setting-row param-mode flex items-center justify-between gap-4 flex-wrap"
              @submit.prevent="setParam(mode)"
            >
              <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
                ><strong
                  ><code
                    class="mode-letter font-code text-[12px] font-[600] py-[1px] px-[5px] mr-1 rounded-[4px] bg-rail text-primary"
                    >+{{ mode }}</code
                  >
                  {{ modeInfo[mode]?.label ?? 'Server-specific mode' }}</strong
                ><small
                  >{{ modeInfo[mode]?.description ?? 'This network defines what this mode does.' }}
                  <template v-if="set.has(mode)"
                    >Currently <strong>{{ mode === 'k' ? 'set' : c.modeParams?.[mode] }}</strong
                    >.</template
                  ></small
                ></span
              >
              <div class="param-controls flex gap-1.5 items-center">
                <UiInput
                  v-model="values[mode]"
                  class="w-40"
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
                /><UiButton
                  variant="secondary"
                  size="sm"
                  :disabled="!rights.op || !valueOf(mode)"
                >
                  Set</UiButton
                ><UiButton
                  variant="secondary"
                  size="sm"
                  v-if="set.has(mode)"
                  type="button"
                  :disabled="!rights.op"
                  @click="clearParam(mode)"
                >
                  Remove
                </UiButton>
              </div>
            </form>
          </section>
          <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
            Registration, founders and access lists are on the Registration tab.
          </p>
        </div>

        <ChannelRegistration
          v-else-if="tab === 'registration'"
          :conversation-id="conversationId"
          :name="name"
          :network-name="network?.name ?? 'This network'"
          :op="rights.op"
          :account="network?.account"
        />

        <!-- List modes -->
        <div
          v-else
          class="appearance-settings grid gap-6"
        >
          <section class="setting-group grid gap-3.5">
            <h3>{{ listInfo[listMode]?.label ?? `+${listMode} list` }}</h3>
            <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5 list-purpose">
              {{ listInfo[listMode]?.description ?? 'A list kept by the server.' }}
              {{ listInfo[listMode]?.overrides }}
            </p>
            <p
              v-if="listMode === 'I' && !set.has('i')"
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
            >
              Invite-only (+i) is off, so this list has no effect until it’s turned on.
            </p>
            <form
              v-if="rights.ban"
              class="mask-add flex gap-2 mask-form"
              @submit.prevent="addMask"
            >
              <div class="mask-field flex-1 min-w-0 grid gap-1">
                <UiInput
                  :ref="(el) => (maskInput = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
                  v-model="newMask"
                  placeholder="alice, or *!*@host.example.net"
                  :aria-label="`Add ${listInfo[listMode]?.singular ?? 'entry'}`"
                  aria-describedby="mask-short-help mask-preview"
                  :aria-invalid="!!preview.error || undefined"
                  autocomplete="off"
                  autocapitalize="none"
                  spellcheck="false"
                />
                <p
                  id="mask-short-help"
                  class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
                >
                  A nick, or a full <code>nick!user@host</code> mask. <code>*</code> matches
                  anything, <code>?</code> one character.
                </p>
                <div
                  id="mask-preview"
                  class="mask-preview text-[0.86rem] text-muted-foreground"
                  aria-live="polite"
                >
                  <p
                    v-if="preview.error"
                    class="form-error text-destructive text-[13px] mt-3"
                  >
                    {{ preview.error }}
                  </p>
                  <template v-else-if="preview.mask">
                    <p>
                      Adds <code>{{ preview.mask }}</code
                      ><template v-if="preview.expanded"> for “{{ newMask.trim() }}”</template>.
                      {{ preview.scope }}
                    </p>
                    <p
                      v-for="warning in preview.warnings"
                      :key="warning"
                      class="mask-warning text-foreground font-[550]"
                    >
                      {{ warning }}
                    </p>
                    <p v-if="matchedHere.length">
                      Matches
                      {{ matchedHere.length === 1 ? 'one person' : `${matchedHere.length} people` }}
                      here now: {{ matchedHere.slice(0, 8).join(', ')
                      }}<template v-if="matchedHere.length > 8">, …</template>
                    </p>
                  </template>
                </div>
              </div>
              <UiButton
                size="sm"
                :disabled="!preview.mask || !!preview.error"
              >
                Add {{ listInfo[listMode]?.singular ?? 'entry' }}
              </UiButton>
            </form>
            <details class="mask-help mt-1 mr-0 mb-2 ml-0 text-[0.88rem] text-muted-foreground">
              <summary>How masks work, with examples</summary>
              <p>A mask has three parts, <code>nick!user@host</code>:</p>
              <dl class="mask-parts grid gap-1.5 my-2 mx-0">
                <div>
                  <dt>nick</dt>
                  <dd>
                    The name people see. Anyone can change nick at any time, so a nick mask follows
                    whoever uses the nick, not a person or their account.
                  </dd>
                </div>
                <div>
                  <dt>user</dt>
                  <dd>
                    The username (ident) their client sends. A leading <code>~</code> means the
                    server couldn’t verify it.
                  </dd>
                </div>
                <div>
                  <dt>host</dt>
                  <dd>
                    Where they connect from, as the network shows it to everyone: an address, a
                    hostname, or a cloak that hides it. <code>/whois nick</code> shows someone’s.
                  </dd>
                </div>
              </dl>
              <p>
                <code>*</code> matches any run of characters (even none) and <code>?</code> exactly
                one; case doesn’t matter. A missing part matches anything: a plain nick
                <code>alice</code> is added as <code>alice!*@*</code>, and
                <code>alice@host</code> as <code>*!alice@host</code>. The broader the mask, the more
                people it catches: <code>*!*@*</code> matches everyone.
              </p>
              <ul class="mask-examples list-none my-2 mx-0 p-0 grid gap-1.5">
                <li
                  v-for="example in examples"
                  :key="example.text"
                >
                  <UiButton
                    variant="secondary"
                    size="sm"
                    v-if="rights.ban"
                    type="button"
                    :aria-label="`Use the example ${example.text}`"
                    @click="useExample(example.text)"
                  >
                    <code>{{ example.text }}</code>
                  </UiButton>
                  <code v-else>{{ example.text }}</code>
                  <span>{{ example.explain }}</span>
                </li>
              </ul>
              <p
                v-if="rights.ban"
                class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >
                Choosing an example only fills in the field; nothing changes until you press Add.
              </p>
              <p v-if="accountPrefix">
                {{ network?.name }} also takes account masks: <code>{{ accountPrefix }}name</code>
                matches whoever is logged in to that services account. Masks without it never
                identify an account.
              </p>
              <p v-else>
                {{ network?.name ?? 'This network' }} doesn’t advertise account masks, so masks here
                only match nick, user and host, and never identify an account.
              </p>
            </details>
            <p
              v-if="list.isLoading.value && !list.data.value"
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
            >
              Loading…
            </p>
            <p
              v-else-if="list.error.value"
              class="form-error text-destructive text-[13px] mt-3"
            >
              {{ errorText(list.error.value) }}
            </p>
            <p
              v-else-if="!list.data.value?.items.length"
              class="list-empty p-6 text-center text-faint text-[13px]"
            >
              Nothing here.
            </p>
            <ul
              v-else
              class="mask-list list-none m-0 p-0 grid gap-0.5"
            >
              <li
                v-for="entry in list.data.value?.items"
                :key="entry.mask"
              >
                <code>{{ entry.mask }}</code>
                <span class="mask-meta ml-auto text-[12px] text-faint whitespace-nowrap"
                  ><template v-if="entry.setBy">by {{ entry.setBy }}</template
                  ><template v-if="entry.setAt"> · {{ date(entry.setAt) }}</template></span
                >
                <UiButton
                  variant="ghost"
                  size="icon-sm"
                  v-if="rights.ban"
                  :aria-label="`Remove ${entry.mask}`"
                  @click="removeMask(entry.mask)"
                >
                  <AppIcon
                    name="trash"
                    :size="15"
                  />
                </UiButton>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </div>
  </AppDialog>
</template>
