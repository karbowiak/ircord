<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { ChannelRights } from '../../shared/utils/irc'
import {
  accountExtban,
  listInfo,
  maskMatches,
  muteExtban,
  parseExtbans,
  previewMask,
} from '../../shared/utils/channel-modes'
import {
  defaultMask,
  knownAccount,
  partsOf,
  toggledMask,
  type MaskPart,
} from '../../shared/utils/mask-builder'
import { channelModeListQuery, messagesQuery } from '../queries/chat'
import type { Conversation, Member, Network } from '../../shared/types/chat'

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
const { name, set, send } = useChannelContext(
  () => props.c,
  () => props.network,
)
const date = channelDate

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
const preview = computed(() => previewMask(newMask.value, extbans.value))
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
    return hit ? [id === 'you' ? `${nick} (you)` : nick] : []
  })
})
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
    { part: 'nick' as MaskPart, label: 'Nick', value: p.nick },
    { part: 'user' as MaskPart, label: 'Username', value: p.username, unknown: 'Not known yet' },
    { part: 'host' as MaskPart, label: 'Host', value: p.host, unknown: 'Not known yet' },
    ...(accountPrefix.value
      ? [
          {
            part: 'account' as MaskPart,
            label: 'Account',
            value: account,
            unknown: accountsTracked.value ? 'Not logged in' : 'Not reported by this network',
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
function choosePerson(m: Member) {
  personId.value = m.id
  chosen.value = m
  newMask.value = defaultMask(m, accountPrefix.value)
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
  ...(mutePrefix.value && props.listMode === 'b'
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
  if (send(`MODE ${name.value} +${props.listMode} ${mask}`) === false) return
  newMask.value = ''
  personId.value = undefined
  refetchSoon()
}
async function removeMask(mask: string) {
  if (
    await confirm({
      title: `Remove ${listInfo[props.listMode]?.singular ?? 'entry'}?`,
      body: mask,
      confirm: 'Remove',
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
      <h3>{{ listInfo[listMode]?.label ?? `+${listMode} list` }}</h3>
      <FieldHint class="list-purpose">
        {{ listInfo[listMode]?.description ?? 'A list kept by the server.' }}
        {{ listInfo[listMode]?.overrides }}
      </FieldHint>
      <FieldHint v-if="listMode === 'I' && !set.has('i')">
        Invite-only (+i) is off, so this list has no effect until it’s turned on.
      </FieldHint>
      <form
        v-if="rights.ban"
        class="mask-form grid gap-3"
        @submit.prevent="addMask"
      >
        <div class="mask-builder grid gap-2">
          <template v-if="!person">
            <MaskPersonPicker
              ref="picker"
              :here="here"
              :recent="recent"
              :channel="name"
              @choose="choosePerson"
            />
            <FieldHint class="m-0">
              Choose someone to build the mask from, then pick what to match them by. To type a mask
              yourself, open Advanced.
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
                :aria-label="`Clear ${person.nick} and start over`"
                @click="clearPerson"
              >
                Clear
              </UiButton>
            </div>
            <fieldset class="mask-parts-choice m-0 p-0 border-0 grid gap-1.5 min-w-0">
              <legend class="mb-1 p-0 text-[13px] font-[550] text-foreground">
                Match {{ person.nick }} by
              </legend>
              <label
                v-for="option in partOptions"
                :key="option.part"
                class="mask-part-option flex items-center gap-2 min-h-8 text-[13px] min-w-0"
                :class="{ 'opacity-60': !option.value }"
                ><UiCheckbox
                  :model-value="!!parts?.[option.part]"
                  :disabled="!option.value"
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
                Unticked parts match anything.<template v-if="accountPrefix">
                  An account can’t be combined with the others: it follows them whatever nick or
                  host they use.</template
                ><template v-if="!person.username || !person.host">
                  Their username and host show up once the network has told IRCord (for example
                  after <code>/whois {{ person.nick }}</code
                  >).</template
                >
              </FieldHint>
              <FieldHint
                v-if="customMask"
                class="m-0 text-foreground"
              >
                The mask was edited under Advanced and no longer names {{ person.nick }}’s own
                parts. Tick a box to build it from them again.
              </FieldHint>
            </fieldset>
          </template>
        </div>
        <details
          :open="advanced"
          @toggle="advanced = ($event.target as HTMLDetailsElement).open"
          class="mask-advanced [&_summary]:w-fit [&_summary]:cursor-pointer [&_summary]:text-[13px] [&_summary]:font-[550] [&_summary]:text-foreground [&_summary:focus-visible]:rounded-[4px] [&_summary:focus-visible]:outline-2 [&_summary:focus-visible]:outline-offset-2 [&_summary:focus-visible]:outline-primary"
        >
          <summary>Advanced: edit the mask</summary>
          <div class="mask-field min-w-0 grid gap-1 mt-2">
            <UiInput
              :ref="(el) => (maskInput = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
              v-model="newMask"
              placeholder="alice, or *!*@host.example.net"
              :aria-label="`Mask for the ${listInfo[listMode]?.singular ?? 'entry'}`"
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
              A nick, or a full <code>nick!user@host</code> mask. <code>*</code> matches anything,
              <code>?</code> one character.
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
        <UiButton
          size="sm"
          class="justify-self-start"
          :disabled="!preview.mask || !!preview.error"
        >
          Add {{ listInfo[listMode]?.singular ?? 'entry' }}
        </UiButton>
      </form>
      <details
        class="mask-help mt-1 mr-0 mb-2 ml-0 text-[0.88rem] text-muted-foreground [&_code]:wrap-anywhere [&_p]:my-2 [&_summary]:w-fit [&_summary]:cursor-pointer [&_summary]:font-[550] [&_summary]:text-foreground [&_summary:focus-visible]:rounded-[4px] [&_summary:focus-visible]:outline-2 [&_summary:focus-visible]:outline-offset-2 [&_summary:focus-visible]:outline-primary"
      >
        <summary>How masks work, with examples</summary>
        <p>A mask has three parts, <code>nick!user@host</code>:</p>
        <dl
          class="mask-parts grid gap-1.5 my-2 mx-0 [&_dd]:m-0 [&_div]:grid [&_div]:[grid-template-columns:3.5rem_1fr] [&_div]:gap-2 [&_dt]:font-code [&_dt]:text-foreground"
        >
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
              The username (ident) their client sends. A leading <code>~</code> means the server
              couldn’t verify it.
            </dd>
          </div>
          <div>
            <dt>host</dt>
            <dd>
              Where they connect from, as the network shows it to everyone: an address, a hostname,
              or a cloak that hides it. <code>/whois nick</code> shows someone’s.
            </dd>
          </div>
        </dl>
        <p>
          <code>*</code> matches any run of characters (even none) and <code>?</code> exactly one;
          case doesn’t matter. A missing part matches anything: a plain nick <code>alice</code> is
          added as <code>alice!*@*</code>, and <code>alice@host</code> as <code>*!alice@host</code>.
          The broader the mask, the more people it catches: <code>*!*@*</code> matches everyone.
        </p>
        <ul
          class="mask-examples list-none my-2 mx-0 p-0 grid gap-1.5 [&_[data-slot=button]]:justify-self-start [&_li]:grid [&_li]:[grid-template-columns:minmax(9rem,auto)_1fr] [&_li]:items-baseline [&_li]:gap-2.5 max-[700px]:[&_li]:[grid-template-columns:1fr] max-[700px]:[&_li]:gap-0.5"
        >
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
        <FieldHint v-if="rights.ban">
          Choosing an example only fills in the field; nothing changes until you press Add.
        </FieldHint>
        <p v-if="accountPrefix">
          {{ network?.name }} also takes account masks: <code>{{ accountPrefix }}name</code>
          matches whoever is logged in to that services account. Masks without it never identify an
          account.
        </p>
        <p v-else>
          {{ network?.name ?? 'This network' }} doesn’t advertise account masks, so masks here only
          match nick, user and host, and never identify an account.
        </p>
      </details>
      <FieldHint v-if="list.isLoading.value && !list.data.value"> Loading… </FieldHint>
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
</template>
