<script setup vapor lang="ts">
import { useQuery } from '@pinia/colada'
import type { ChannelRights } from '../../../shared/utils/irc'
import {
  accountExtban,
  listInfo,
  maskMatches,
  muteExtban,
  parseExtbans,
  previewMask,
} from '../../../shared/utils/channel-modes'
import {
  defaultMask,
  knownAccount,
  partsOf,
  toggledMask,
  type MaskPart,
} from '../../../shared/utils/mask-builder'
import { channelModeListQuery, messagesQuery } from '../../queries/chat'
import type { Conversation, Member, Network } from '../../../shared/types/chat'

/** Channel settings → one list mode (bans, exceptions, invites, quiets): its entries and adding. */
const props = defineProps<{
  c: Conversation
  network?: Network
  rights: ChannelRights
  /** The list's mode letter, such as b. */
  listMode: string
}>()
const chat = useChatStore()
const { confirm } = usePrompt()
const { t, n } = useI18n()
const { day } = useTimeFormat()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const translate = (key: string, values?: Record<string, string | number>) => t(key, values ?? {})
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const date = (iso: string) => day(new Date(iso).toISOString())
const lists = computed(() => listInfo(translate))

const list = useQuery(() =>
  channelModeListQuery({ conversationId: props.c.id, mode: props.listMode }),
)
const newMask = ref('')
let refresh: ReturnType<typeof setTimeout> | undefined
/** List changes arrive as mode events; refetch shortly after sending one. */
function refetchSoon() {
  clearTimeout(refresh)
  refresh = setTimeout(() => void list.refetch(), 400)
}
const extbans = computed(() => parseExtbans(props.network?.isupport))
/** What Add will send, shown before it's sent. */
const preview = computed(() => previewMask(newMask.value, extbans.value, translate))
const accountPrefix = computed(() => accountExtban(extbans.value))
const mutePrefix = computed(() => muteExtban(extbans.value))
/**
 * Who in the channel the mask would match right now. People whose username and host aren't
 * known yet only count when the mask doesn't look at them.
 */
const matchedHere = computed(() => {
  const mask = preview.value.mask
  if (!mask || preview.value.extban) return []
  const [nickPart = '*', rest = '*@*'] = mask.split('!')
  const loose = /^[*?]*@[*?]*$/.test(rest)
  return props.c.memberIds.flatMap((id) => {
    const m = chat.member(id)
    const nick = id === 'you' ? (props.network?.nick ?? m?.nick) : m?.nick
    if (!nick) return []
    const hit =
      m?.username && m.host
        ? maskMatches(mask, `${nick}!${m.username}@${m.host}`)
        : loose && maskMatches(nickPart, nick)
    return hit ? [id === 'you' ? t('collectionsUi.channel.maskUi.youNick', { nick }) : nick] : []
  })
})
const matchedNames = computed(
  () => `${matchedHere.value.slice(0, 8).join(', ')}${matchedHere.value.length > 8 ? ', …' : ''}`,
)
const maskInput = ref<HTMLInputElement | null>(null)
/** The raw mask field, folded away while a mask is built from someone. */
const advanced = ref(false)
/** Examples only fill the field; nothing is sent until Add. */
function useExample(text: string) {
  personId.value = undefined
  newMask.value = text
  advanced.value = true
  nextTick(() => maskInput.value?.focus())
}

// ---- Building a mask from someone ----------------------------------------------------------
// The mask field stays the source of truth: the boxes are read back from it, so editing it by
// hand under Advanced and ticking boxes always agree.
const history = useQuery(() => messagesQuery(props.c.id))
const here = computed(() =>
  props.c.memberIds
    .filter((id) => id !== 'you')
    .map((id) => chat.member(id))
    .filter((m): m is Member => !!m),
)
/** People who spoke, joined or left in the loaded history and aren't in the channel now. */
const recent = computed(() => {
  const present = new Set(props.c.memberIds)
  const seen = new Set<string>()
  const people: Member[] = []
  const items = history.data.value?.items ?? []
  for (let i = items.length - 1; i >= 0 && people.length < 50; i--) {
    const id = items[i]!.senderId
    if (!id || id === 'you' || present.has(id) || seen.has(id)) continue
    seen.add(id)
    const m = chat.member(id)
    if (m?.nick) people.push(m)
  }
  return people
})
const personId = ref<string>()
/** Kept so the person stays chosen after they leave or their entry goes away. */
const chosen = ref<Member>()
const person = computed(() =>
  personId.value ? (chat.member(personId.value) ?? chosen.value) : undefined,
)
const accountsTracked = computed(() => !!props.network?.capabilities?.includes('account-notify'))
const parts = computed(() =>
  person.value ? partsOf(newMask.value, person.value, accountPrefix.value) : undefined,
)
const partOptions = computed(() => {
  const p = person.value
  if (!p) return []
  const account = knownAccount(p)
  return [
    { part: 'nick' as MaskPart, label: t('collectionsUi.channel.maskUi.nick'), value: p.nick },
    {
      part: 'user' as MaskPart,
      label: t('collectionsUi.channel.maskUi.username'),
      value: p.username,
      unknown: t('collectionsUi.channel.maskUi.notKnown'),
    },
    {
      part: 'host' as MaskPart,
      label: t('collectionsUi.channel.maskUi.host'),
      value: p.host,
      unknown: t('collectionsUi.channel.maskUi.notKnown'),
    },
    ...(accountPrefix.value
      ? [
          {
            part: 'account' as MaskPart,
            label: t('collectionsUi.channel.maskUi.account'),
            value: account,
            unknown: t(
              accountsTracked.value
                ? 'collectionsUi.channel.maskUi.notLoggedIn'
                : 'collectionsUi.channel.maskUi.notReported',
            ),
          },
        ]
      : []),
  ]
})
/** The mask no longer pins any of the chosen person's own parts (edited by hand). */
const customMask = computed(
  () => !!parts.value && !!newMask.value.trim() && !Object.values(parts.value).some(Boolean),
)
const picker = ref<{ focus: () => void } | null>(null)
/** Ticked parts; the last one can't be unticked, or the builder would make `*!*@*`. */
const tickedCount = computed(() =>
  parts.value ? Object.values(parts.value).filter(Boolean).length : 0,
)
const lastTicked = (part: MaskPart) => tickedCount.value === 1 && !!parts.value?.[part]
const partsChoice = ref<HTMLElement | null>(null)
function choosePerson(m: Member) {
  personId.value = m.id
  chosen.value = m
  newMask.value = defaultMask(m, accountPrefix.value)
  // The search field goes away; hand focus to the first box so keyboard users carry on there.
  nextTick(() =>
    partsChoice.value
      ?.querySelector<HTMLElement>('[role="checkbox"]:not([disabled]):not([data-disabled])')
      ?.focus(),
  )
}
function clearPerson() {
  personId.value = undefined
  newMask.value = ''
  nextTick(() => picker.value?.focus())
}
function togglePart(part: MaskPart, on: boolean) {
  if (!person.value) return
  newMask.value = toggledMask(newMask.value, person.value, part, on, accountPrefix.value)
}
const examples = computed(() => [
  {
    text: 'alice',
    explain: t('collectionsUi.channel.maskUi.exampleNick'),
  },
  {
    text: '*!*@203.0.113.7',
    explain: t('collectionsUi.channel.maskUi.exampleAddress'),
  },
  {
    text: '*!*@*.example.net',
    explain: t('collectionsUi.channel.maskUi.exampleHost'),
  },
  {
    text: '*!*alice@*',
    explain: t('collectionsUi.channel.maskUi.exampleUsername'),
  },
  ...(accountPrefix.value
    ? [
        {
          text: `${accountPrefix.value}alice`,
          explain: t('collectionsUi.channel.maskUi.exampleAccount'),
        },
      ]
    : []),
  ...(mutePrefix.value && props.listMode === 'b'
    ? [
        {
          text: 'm:alice',
          explain: t('collectionsUi.channel.maskUi.exampleMute'),
        },
      ]
    : []),
])
function addMask() {
  const { mask, error } = preview.value
  if (!mask || error) return
  // Not sent (disconnected): keep what was typed so it can be added once reconnected.
  if (send(`MODE ${name.value} +${props.listMode} ${mask}`) === false) return
  newMask.value = ''
  personId.value = undefined
  refetchSoon()
}
async function removeMask(mask: string) {
  if (
    await confirm({
      title: t('collectionsUi.channel.maskUi.removeListItem', {
        entry: lists.value[props.listMode]?.singular ?? t('collectionsUi.channel.maskUi.entry'),
      }),
      body: mask,
      confirm: t('common.remove'),
    })
  ) {
    send(`MODE ${name.value} -${props.listMode} ${mask}`)
    refetchSoon()
  }
}
onBeforeUnmount(() => clearTimeout(refresh))
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <section class="setting-group grid gap-3.5">
      <h3>
        {{
          lists[listMode]?.label ?? t('collectionsUi.channel.listModeFallback', { mode: listMode })
        }}
      </h3>
      <FieldHint class="list-purpose">
        {{ lists[listMode]?.description ?? t('collectionsUi.channel.list.serverListDescription') }}
        {{ lists[listMode]?.overrides }}
      </FieldHint>
      <FieldHint v-if="listMode === 'I' && !set.has('i')">
        {{ t('collectionsUi.channel.maskUi.inviteOnlyOff') }}
      </FieldHint>
      <form
        v-if="rights.ban"
        class="mask-form grid gap-3 rounded-[var(--radius-large)] bg-card p-4 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
        @submit.prevent="addMask"
      >
        <div class="mask-builder grid gap-2">
          <template v-if="!person">
            <MaskPersonPicker
              ref="picker"
              :here="here"
              :recent="recent"
              :channel="name"
              :case-mapping="network?.isupport?.CASEMAPPING"
              :network-id="c.networkId"
              :conversation-id="c.id"
              @choose="choosePerson"
            />
            <FieldHint class="m-0">
              {{ t('collectionsUi.channel.maskUi.choosePerson') }}
            </FieldHint>
          </template>
          <template v-else>
            <div
              class="mask-person flex items-center gap-2.5 min-w-0 rounded-md border border-solid border-border bg-secondary py-2 px-2.5"
            >
              <UserAvatar
                :member="person"
                size="small"
              />
              <span class="grid min-w-0 flex-1 gap-[1px]">
                <span class="font-[550] text-foreground wrap-anywhere">{{ person.nick }}</span>
                <span
                  v-if="person.displayName && person.displayName !== person.nick"
                  class="text-[0.82em] text-muted-foreground wrap-anywhere"
                  >{{ person.displayName }}</span
                >
              </span>
              <UiButton
                variant="ghost"
                size="sm"
                type="button"
                :aria-label="t('collectionsUi.channel.maskUi.clearPerson', { nick: person.nick })"
                @click="clearPerson"
              >
                {{ t('collectionsUi.channel.maskUi.clear') }}
              </UiButton>
            </div>
            <fieldset
              ref="partsChoice"
              class="mask-parts-choice m-0 p-0 border-0 grid gap-1.5 min-w-0"
            >
              <legend class="mb-1 p-0 text-[13px] font-[550] text-foreground">
                {{ t('collectionsUi.channel.maskUi.matchPersonBy', { nick: person.nick }) }}
              </legend>
              <label
                v-for="option in partOptions"
                :key="option.part"
                class="mask-part-option flex items-center gap-2 min-h-8 text-[13px] min-w-0"
                :class="{ 'opacity-60': !option.value }"
                ><UiCheckbox
                  :model-value="!!parts?.[option.part]"
                  :disabled="!option.value || lastTicked(option.part)"
                  @update:model-value="togglePart(option.part, !!$event)"
                /><span class="w-[5.5rem] shrink-0 text-foreground">{{ option.label }}</span
                ><code
                  v-if="option.value"
                  class="min-w-0 wrap-anywhere text-muted-foreground"
                  >{{ option.value }}</code
                ><span
                  v-else
                  class="text-muted-foreground"
                  >{{ option.unknown }}</span
                ></label
              >
              <FieldHint class="m-0">
                {{ t('collectionsUi.channel.maskUi.uncheckedParts')
                }}<template v-if="accountPrefix">
                  {{ t('collectionsUi.channel.maskUi.accountAlone') }}</template
                ><template v-if="!person.username || !person.host">
                  {{
                    t('collectionsUi.channel.maskUi.identityMayBeUnknown', {
                      nick: person.nick,
                      command: '/whois ' + person.nick,
                    })
                  }}</template
                >
              </FieldHint>
              <FieldHint
                v-if="customMask"
                class="m-0 text-foreground"
              >
                {{ t('collectionsUi.channel.maskUi.customMask', { nick: person.nick }) }}
              </FieldHint>
            </fieldset>
          </template>
        </div>
        <details
          class="mask-advanced [&_summary]:w-fit [&_summary]:cursor-pointer [&_summary]:text-[13px] [&_summary]:font-[550] [&_summary]:text-foreground [&_summary:focus-visible]:rounded-[4px] [&_summary:focus-visible]:outline-2 [&_summary:focus-visible]:outline-offset-2 [&_summary:focus-visible]:outline-primary"
          :open="advanced"
          @toggle="advanced = ($event.target as HTMLDetailsElement).open"
        >
          <summary>{{ t('collectionsUi.channel.maskUi.advanced') }}</summary>
          <div class="mask-field min-w-0 grid gap-1 mt-2">
            <UiInput
              :ref="(el) => (maskInput = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
              v-model="newMask"
              :placeholder="t('collectionsUi.channel.maskUi.maskPlaceholder')"
              :aria-label="
                t('collectionsUi.channel.maskUi.maskFor', {
                  entry: lists[listMode]?.singular ?? t('collectionsUi.channel.maskUi.entry'),
                })
              "
              aria-describedby="mask-short-help mask-preview"
              :aria-invalid="!!preview.error || undefined"
              autocomplete="off"
              autocapitalize="none"
              spellcheck="false"
            />
            <FieldHint
              id="mask-short-help"
              class="m-0"
            >
              {{
                t('collectionsUi.channel.maskUi.maskShortHelp', {
                  mask: 'nick!user@host',
                  any: '*',
                  one: '?',
                })
              }}
            </FieldHint>
          </div>
        </details>
        <div
          id="mask-preview"
          class="mask-preview text-[0.86rem] text-muted-foreground [&_code]:wrap-anywhere [&_p]:mt-0.5 [&_p]:mb-0"
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
              {{
                t(
                  preview.expanded
                    ? 'collectionsUi.channel.maskUi.addExpanded'
                    : 'collectionsUi.channel.maskUi.addMask',
                  { mask: preview.mask, typed: newMask.trim(), scope: preview.scope ?? '' },
                )
              }}
            </p>
            <p
              v-for="warning in preview.warnings"
              :key="warning"
              class="mask-warning text-foreground font-[550]"
            >
              {{ warning }}
            </p>
            <p v-if="matchedHere.length">
              {{
                t(
                  'collectionsUi.channel.maskUi.matchesHere',
                  { count: n(matchedHere.length), people: matchedNames },
                  matchedHere.length,
                )
              }}
            </p>
          </template>
        </div>
        <UiButton
          size="sm"
          class="justify-self-start"
          :disabled="!preview.mask || !!preview.error"
        >
          {{
            t('collectionsUi.channel.maskUi.addListItem', {
              entry: lists[listMode]?.singular ?? t('collectionsUi.channel.maskUi.entry'),
            })
          }}
        </UiButton>
      </form>
      <details
        class="mask-help mt-1 mr-0 mb-2 ml-0 text-[0.88rem] text-muted-foreground [&_code]:wrap-anywhere [&_p]:my-2 [&_summary]:w-fit [&_summary]:cursor-pointer [&_summary]:font-[550] [&_summary]:text-foreground [&_summary:focus-visible]:rounded-[4px] [&_summary:focus-visible]:outline-2 [&_summary:focus-visible]:outline-offset-2 [&_summary:focus-visible]:outline-primary"
      >
        <summary>{{ t('collectionsUi.channel.maskUi.howMasksWork') }}</summary>
        <p>{{ t('collectionsUi.channel.maskUi.threeParts', { mask: 'nick!user@host' }) }}</p>
        <dl
          class="mask-parts grid gap-1.5 my-2 mx-0 [&_dd]:m-0 [&_div]:grid [&_div]:[grid-template-columns:3.5rem_1fr] [&_div]:gap-2 [&_dt]:font-code [&_dt]:text-foreground"
        >
          <div>
            <dt>nick</dt>
            <dd>
              {{ t('collectionsUi.channel.maskUi.nickExplanation') }}
            </dd>
          </div>
          <div>
            <dt>user</dt>
            <dd>
              {{ t('collectionsUi.channel.maskUi.userExplanation', { tilde: '~' }) }}
            </dd>
          </div>
          <div>
            <dt>host</dt>
            <dd>
              {{ t('collectionsUi.channel.maskUi.hostExplanation', { command: '/whois nick' }) }}
            </dd>
          </div>
        </dl>
        <p>
          {{ t('collectionsUi.channel.maskUi.wildcards', { any: '*', one: '?' }) }}
          {{
            t('collectionsUi.channel.maskUi.missingParts', {
              nick: 'alice',
              nickMask: 'alice!*@*',
              userHost: 'alice@host',
              userHostMask: '*!alice@host',
              everyoneMask: '*!*@*',
            })
          }}
        </p>
        <ul
          class="mask-examples list-none my-2 mx-0 p-0 grid gap-1.5 [&_[data-slot=button]]:justify-self-start [&_li]:grid [&_li]:[grid-template-columns:minmax(9rem,auto)_1fr] [&_li]:items-baseline [&_li]:gap-2.5 max-[700px]:[&_li]:[grid-template-columns:1fr] max-[700px]:[&_li]:gap-0.5"
        >
          <li
            v-for="example in examples"
            :key="example.text"
          >
            <UiButton
              v-if="rights.ban"
              variant="secondary"
              size="sm"
              type="button"
              :aria-label="t('collectionsUi.channel.maskUi.useExample', { example: example.text })"
              @click="useExample(example.text)"
            >
              <code>{{ example.text }}</code>
            </UiButton>
            <code v-else>{{ example.text }}</code>
            <span>{{ example.explain }}</span>
          </li>
        </ul>
        <FieldHint v-if="rights.ban">
          {{ t('collectionsUi.channel.maskUi.exampleOnly') }}
        </FieldHint>
        <p v-if="accountPrefix">
          {{
            t('collectionsUi.channel.maskUi.accountMasksSupported', {
              network: network?.name ?? '',
              mask: accountPrefix + 'name',
            })
          }}
        </p>
        <p v-else>
          {{
            t('collectionsUi.channel.maskUi.accountMasksUnsupported', {
              network: network?.name ?? t('collectionsUi.channel.settings.thisNetwork'),
            })
          }}
        </p>
      </details>
      <FieldHint v-if="list.isLoading.value && !list.data.value">{{
        t('collectionsUi.channel.maskUi.loading')
      }}</FieldHint>
      <p
        v-else-if="list.error.value"
        class="form-error text-destructive text-[13px] mt-3"
      >
        {{
          t('collectionsUi.channel.maskUi.loadFailed', { error: errorMessage(list.error.value) })
        }}
      </p>
      <EmptyState
        v-else-if="!list.data.value?.items.length"
        class="list-empty py-8"
        icon="shield"
      >
        <p>{{ t('collectionsUi.channel.maskUi.empty') }}</p>
      </EmptyState>
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
            ><template v-if="entry.setBy">{{
              t('collectionsUi.channel.maskUi.setBy', { person: entry.setBy })
            }}</template
            ><template v-if="entry.setAt"> · {{ date(entry.setAt) }}</template></span
          >
          <UiButton
            v-if="rights.ban"
            variant="ghost"
            size="icon-sm"
            :aria-label="t('collectionsUi.channel.maskUi.removeMask', { mask: entry.mask })"
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
</template>
