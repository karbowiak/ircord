<script setup vapor lang="ts">
import {
  accountSuggestions,
  highlightIndex,
  nickWithOtherAccount,
  type AccountSuggestion,
  type RosterPerson,
} from '../../shared/utils/account-suggestions'

/**
 * A text field for a services account with suggestions from the people in a channel (the
 * WAI-ARIA combobox pattern with list autocomplete). Choosing someone fills in their account and
 * nothing else; any other account name can still be typed. The hint below always names the
 * account the form will act on.
 */
const props = withDefaults(
  defineProps<{
    people: readonly RosterPerson[]
    /** The channel the people are in, for the hint text. */
    channel: string
    label: string
    placeholder?: string
    disabled?: boolean
    /** The network reports accounts (account-notify), so a missing one means "not logged in". */
    accountsTracked?: boolean
    exclude?: readonly string[]
    /** Extra context for an account, such as the role it already has. */
    note?: (account: string) => string | undefined
  }>(),
  { placeholder: undefined, disabled: false, accountsTracked: true, exclude: () => [] },
)
const model = defineModel<string>({ required: true })
const { t, n } = useI18n()
const networkHint = computed(() => t('collectionsUi.accountPicker.networkHint'))

const listId = useId()
const hintId = useId()
const optionId = (index: number) => `${listId}-${index}`
const input = ref<HTMLInputElement | null>(null)
const list = ref<HTMLElement | null>(null)
const open = ref(false)
const position = ref<Record<string, string>>({})

const results = computed(() =>
  accountSuggestions(props.people, model.value, { exclude: props.exclude }),
)
const options = computed(() => results.value.accounts)
const unknown = computed(() => results.value.unknown)
const visible = computed(() => open.value && (options.value.length > 0 || unknown.value.length > 0))

// The highlight follows an account, not a position: when the roster changes under an open list,
// it stays on the same person or clears, and never slides onto someone else.
const activeAccount = ref<string>()
const active = computed({
  get: () => highlightIndex(options.value, activeAccount.value),
  set: (index: number) => (activeAccount.value = options.value[index]?.account),
})
watch([options, unknown], () => {
  if (active.value < 0) activeAccount.value = undefined
  if (visible.value) nextTick(place)
})
watch(
  () => props.disabled,
  (disabled) => disabled && close(),
)

function place() {
  if (!input.value) return
  const rect = input.value.getBoundingClientRect()
  const rows = options.value.length + unknown.value.length
  const p = placePopup(
    rect,
    { width: window.innerWidth, height: window.innerHeight },
    Math.min(320, rows * 44 + 10),
  )
  position.value = {
    left: `${p.left}px`,
    minWidth: `${p.width}px`,
    maxWidth: `${Math.max(p.width, Math.min(window.innerWidth - 16, 420))}px`,
    maxHeight: `${p.maxHeight}px`,
    ...(p.up ? { bottom: `${p.bottom}px`, top: 'auto' } : { top: `${p.top}px`, bottom: 'auto' }),
  }
}
async function show() {
  if (props.disabled) return
  open.value = true
  await nextTick()
  if (!visible.value) return
  place()
  if (list.value && !list.value.matches?.(':popover-open')) list.value.showPopover?.()
}
function close() {
  open.value = false
  active.value = -1
}
watch(visible, (value) => {
  if (value) void show()
})

function choose(item: AccountSuggestion) {
  model.value = item.account
  close()
  input.value?.focus({ preventScroll: true })
}
function scrollActive() {
  nextTick(() =>
    list.value
      ?.querySelector(`#${CSS.escape(optionId(active.value))}`)
      ?.scrollIntoView({ block: 'nearest' }),
  )
}
function onInput() {
  active.value = -1
  void show()
}
function onKeydown(event: KeyboardEvent) {
  if (event.isComposing) return
  const count = options.value.length
  switch (event.key) {
    case 'ArrowDown':
    case 'ArrowUp': {
      event.preventDefault()
      if (!open.value) {
        void show()
        if (count) active.value = event.key === 'ArrowDown' ? 0 : count - 1
      } else if (count) {
        const step = event.key === 'ArrowDown' ? 1 : -1
        active.value =
          active.value < 0 && step < 0 ? count - 1 : (active.value + step + count) % count
      }
      scrollActive()
      break
    }
    case 'Enter':
      // Enter picks the highlighted person; with nothing highlighted it submits the form.
      if (visible.value && active.value >= 0 && options.value[active.value]) {
        event.preventDefault()
        choose(options.value[active.value]!)
      } else close()
      break
    case 'Escape':
      // Close the list without closing the dialog around it.
      if (visible.value) {
        event.preventDefault()
        event.stopPropagation()
        close()
      }
      break
    case 'Tab':
      close()
      break
  }
}
function onOutside(event: Event) {
  const target = event.target as Node | null
  if (target && (input.value?.contains(target) || list.value?.contains(target))) return
  close()
}
function onViewportChange(event: Event) {
  if (event.type === 'scroll' && list.value?.contains(event.target as Node)) return
  place()
}
watch(visible, (value) => {
  const method = value ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onOutside, true)
  window[method]('resize', onViewportChange)
  window[method]('scroll', onViewportChange, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onOutside, true)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})

const typed = computed(() => model.value.trim())
/** The person in the channel with exactly the typed account, for context. */
const exact = computed(() =>
  options.value.find((o) => o.account.toLowerCase() === typed.value.toLowerCase()),
)
/** The typed text is someone's nick, but their account is a different one (or unknown). */
const mismatch = computed(() =>
  exact.value ? undefined : nickWithOtherAccount(props.people, typed.value),
)
const others = (item: AccountSuggestion) =>
  item.nicks.filter((n) => n.toLowerCase() !== item.account.toLowerCase())
</script>

<template>
  <div class="account-combobox flex-1 min-w-0 grid gap-1 max-[700px]:basis-full">
    <UiInput
      :ref="(el) => (input = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
      v-model="model"
      type="text"
      role="combobox"
      :class="{ open: visible }"
      :placeholder="placeholder ?? t('collectionsUi.accountPicker.accountName')"
      :aria-label="label"
      aria-autocomplete="list"
      :aria-expanded="visible"
      :aria-controls="visible ? listId : undefined"
      :aria-activedescendant="visible && active >= 0 ? optionId(active) : undefined"
      :aria-describedby="hintId"
      :disabled="disabled"
      autocomplete="off"
      autocapitalize="none"
      spellcheck="false"
      @input="onInput"
      @click="show()"
      @keydown="onKeydown"
      @blur="close"
    />
    <ul
      v-if="visible"
      :id="listId"
      ref="list"
      popover="manual"
      role="listbox"
      class="app-select-list fixed inset-auto m-0 p-1 overflow-y-auto overscroll-contain list-none bg-secondary text-foreground border border-solid border-border rounded-md shadow-popover text-[13px] account-suggestions"
      :aria-label="t('collectionsUi.accountPicker.peopleIn', { channel })"
      :style="position"
      tabindex="-1"
      @pointerdown.prevent
    >
      <li
        v-for="(item, index) in options"
        :id="optionId(index)"
        :key="item.account"
        role="option"
        class="app-select-option account-option flex py-1.5 px-2 rounded-sm cursor-pointer wrap-anywhere flex-col items-start justify-center gap-[1px] min-h-10 [&.active]:bg-accent aria-selected:font-[550] aria-selected:text-foreground"
        :class="{ active: index === active }"
        :aria-selected="index === active"
        @pointermove="active = index"
        @click="choose(item)"
      >
        <span class="account-option-name font-[550] text-foreground">{{ item.account }}</span>
        <span class="account-option-meta text-[0.82em] text-muted-foreground"
          ><template v-if="others(item).length">{{
            t('collectionsUi.accountPicker.inChannelAs', {
              channel,
              nicks: others(item).join(', '),
            })
          }}</template
          ><template v-else-if="item.nicks.length > 1">{{
            t(
              'collectionsUi.accountPicker.sessions',
              { count: n(item.nicks.length) },
              item.nicks.length,
            )
          }}</template
          ><template v-if="item.name"> · {{ item.name }}</template
          ><template v-if="note?.(item.account)"> · {{ note(item.account) }}</template></span
        >
      </li>
      <li
        v-for="person in unknown"
        :key="`unknown:${person.nick}`"
        role="option"
        aria-disabled="true"
        aria-selected="false"
        class="app-select-option account-option flex py-1.5 px-2 rounded-sm wrap-anywhere flex-col items-start justify-center gap-[1px] min-h-10 unknown cursor-default opacity-80"
      >
        <span class="account-option-name font-[450] text-foreground">{{ person.nick }}</span>
        <span class="account-option-meta text-[0.82em] text-muted-foreground">{{
          t(
            accountsTracked
              ? 'collectionsUi.accountPicker.notLoggedIn'
              : 'collectionsUi.accountPicker.notReported',
          )
        }}</span>
      </li>
    </ul>
    <FieldHint
      :id="hintId"
      class="account-target m-0 [&_strong]:font-[600] [&_strong]:text-foreground [&_strong]:wrap-anywhere"
      aria-live="polite"
    >
      <template v-if="!typed">
        <template v-if="people.length">{{
          t('collectionsUi.accountPicker.typeAccountOrNick', { channel })
        }}</template>
        <template v-else>{{ t('collectionsUi.accountPicker.typeAccount') }}</template>
      </template>
      <template v-else-if="mismatch?.account">{{
        t('collectionsUi.accountPicker.nickAccountMismatch', {
          nick: mismatch.nick,
          account: mismatch.account,
        })
      }}</template>
      <template v-else-if="mismatch && accountsTracked">{{
        t('collectionsUi.accountPicker.nickNotLoggedIn', { nick: mismatch.nick, account: typed })
      }}</template>
      <template v-else-if="mismatch">{{
        t('collectionsUi.accountPicker.nickAccountUnverified', {
          hint: networkHint,
          account: typed,
          nick: mismatch.nick,
        })
      }}</template>
      <template v-else-if="exact">{{
        t('collectionsUi.accountPicker.exactAccount', {
          account: exact.account,
          nicks: exact.nicks.join(', '),
          channel,
        })
      }}</template>
      <template v-else>{{
        t('collectionsUi.accountPicker.typedAccount', { account: typed, channel })
      }}</template>
    </FieldHint>
  </div>
</template>
