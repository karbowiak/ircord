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
import { channelModeListQuery } from '../queries/chat'
import type { Conversation, Network } from '../../shared/types/chat'

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
          <FieldHint
            id="mask-short-help"
            class="m-0"
          >
            A nick, or a full <code>nick!user@host</code> mask. <code>*</code> matches anything,
            <code>?</code> one character.
          </FieldHint>
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
        </div>
        <UiButton
          size="sm"
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
              v-if="rights.ban"
              variant="secondary"
              size="sm"
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
            v-if="rights.ban"
            variant="ghost"
            size="icon-sm"
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
