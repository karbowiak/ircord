<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { CommandArgumentSpec } from '../../../shared/utils/commands'
import type { EmojiItem } from '../../../shared/types/media'
import { emojiText, rankEmoji, searchEmoji } from '../../../shared/utils/media'
import { matchingPeople } from '../../../shared/utils/people'
import { channelListQuery } from '../../queries/chat'

const props = defineProps<{
  name: string
  fields: readonly CommandArgumentSpec[]
  initial?: Record<string, string>
  compact?: boolean
  busy?: boolean
  error?: string
  submitLabel?: string
  busyLabel?: string
  title?: string
  keepLabel?: string
  changedKeepLabel?: string
}>()
const emit = defineEmits<{ submit: [values: Record<string, string>]; cancel: []; clear: [] }>()
const chat = useChatStore()
const media = useMediaStore()
const library = useLibraryStore()
const notifications = useNotificationStore()
const { t } = useI18n()
const values = reactive<Record<string, string>>({ ...props.initial })
const initialChanged = (except: string) =>
  Object.entries(props.initial ?? {}).some(
    ([name, value]) => name !== except && values[name] !== value,
  )
const focused = ref(0)
const active = ref(0)
const searching = ref(false)
const listId = useId()
const root = ref<HTMLElement | null>(null)
const field = computed(() => props.fields[focused.value])
const provider = (item: CommandArgumentSpec | undefined) =>
  item?.autocomplete ??
  (item?.type === 'nickname' || item?.type === 'channel' || item?.type === 'emoji'
    ? item.type
    : undefined)
const query = computed(() => (field.value ? (values[field.value.name] ?? '') : ''))
const directoryQuery = ref('')
const { start, stop } = useTimeoutFn((q: string) => (directoryQuery.value = q), 200, {
  immediate: false,
})
watch([field, query], ([current, q]) => {
  active.value = 0
  stop()
  if (provider(current) === 'channel' && q.trim()) start(q.replace(/^#/, ''))
})
const directory = useQuery(() => ({
  ...channelListQuery({ networkId: chat.networkId, q: directoryQuery.value }),
  enabled: provider(field.value) === 'channel' && !!directoryQuery.value,
}))
type Suggestion = { value: string; label: string; emoji?: EmojiItem; detail?: string }
const suggestions = computed<Suggestion[]>(() => {
  if (!searching.value || !field.value) return []
  const q = query.value
    .trim()
    .replace(/^[:@#]/, '')
    .replace(/:$/, '')
  switch (provider(field.value)) {
    case 'emoji':
      if (!q) return []
      return rankEmoji(searchEmoji(media.emojis, q), library.usageRank, q)
        .slice(0, 8)
        .map((item) => ({
          value: emojiText(item, library.library.skinTone),
          label: `:${item.shortcode}:`,
          emoji: item,
        }))
    case 'nickname':
      return matchingPeople(
        (chat.active?.memberIds ?? [])
          .map((id) => chat.member(id))
          .filter((m): m is NonNullable<typeof m> => !!m && m.id !== 'you'),
        q,
        chat.network?.isupport?.CASEMAPPING,
      )
        .slice(0, 12)
        .map((member) => ({ value: member.nick, label: member.nick, detail: member.displayName }))
    case 'ignored':
      return notifications.settings.ignore
        .filter((value) => value.toLowerCase().includes(q.toLowerCase()))
        .slice(0, 12)
        .map((value) => ({ value, label: value }))
    case 'channel': {
      const joined = (chat.data?.conversations ?? [])
        .filter(
          (c) =>
            c.networkId === chat.networkId &&
            c.kind === 'channel' &&
            !c.party &&
            c.name.toLowerCase().includes(q.toLowerCase()),
        )
        .map((c) => ({
          value: `#${c.name.replace(/^#/, '')}`,
          label: `#${c.name.replace(/^#/, '')}`,
        }))
      const known = new Set(joined.map((c) => c.value.toLowerCase()))
      const listed = (q ? (directory.data.value?.items ?? []) : [])
        .map((c) => ({
          value: `#${c.name.replace(/^#/, '')}`,
          label: `#${c.name.replace(/^#/, '')}`,
        }))
        .filter(
          (c) =>
            !known.has(c.value.toLowerCase()) && c.value.toLowerCase().includes(q.toLowerCase()),
        )
      return [...joined, ...listed].slice(0, 12)
    }
    default:
      return []
  }
})
const suggestionsOpen = computed(() => suggestions.value.length > 0)
function focusField(index: number) {
  focused.value = index
  searching.value = false
}
function inputField(event: Event) {
  searching.value = !(event as InputEvent).isComposing
  active.value = 0
}
function focus(index = 0) {
  nextTick(() => {
    const fields = root.value?.querySelectorAll<HTMLInputElement | HTMLSelectElement>(
      '[data-command-argument]',
    )
    fields?.item(index)?.focus()
  })
}
function choose(item: Suggestion) {
  if (!field.value) return
  values[field.value.name] = item.value
  if (item.emoji) library.useEmoji(item.emoji)
  searching.value = false
}
function keydown(event: KeyboardEvent) {
  if (event.isComposing) return
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    if (suggestionsOpen.value) searching.value = false
    else emit('cancel')
    return
  }
  if (suggestionsOpen.value && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
    event.preventDefault()
    active.value =
      (active.value + (event.key === 'ArrowDown' ? 1 : -1) + suggestions.value.length) %
      suggestions.value.length
    return
  }
  if (
    suggestionsOpen.value &&
    (event.key === 'Enter' || (event.key === 'Tab' && !event.shiftKey))
  ) {
    event.preventDefault()
    choose(suggestions.value[active.value]!)
    if (event.key === 'Tab') {
      if (focused.value < props.fields.length - 1) focus(focused.value + 1)
      else
        nextTick(() =>
          root.value?.querySelector<HTMLButtonElement>('button[type="submit"]')?.focus(),
        )
    }
  }
}
onMounted(() => {
  if (props.fields.some((f) => provider(f) === 'emoji')) media.needAll()
  focus()
})
defineExpose({ focus, suggestionsOpen })
</script>

<template>
  <section
    ref="root"
    data-command-editor
    :data-command-name="name"
    class="relative text-foreground"
    @keydown="keydown"
  >
    <div class="absolute bottom-full left-1 right-1 z-30">
      <p
        v-if="compact && error"
        role="alert"
        class="m-0 rounded-t-md bg-card px-3 py-1.5 text-[length:var(--ui-small)] text-destructive shadow-popover"
      >
        {{ error }}
      </p>
      <div
        v-if="suggestionsOpen"
        class="max-h-[min(280px,35dvh)] overflow-y-auto rounded-md bg-card p-1.5 shadow-popover"
        :data-status-emoji-suggestions="
          name === 'status' && provider(field) === 'emoji' ? '' : undefined
        "
      >
        <ul
          :id="listId"
          role="listbox"
          :aria-label="t('workspaceUi.commandEditor.suggestions')"
          class="m-0 list-none p-0"
        >
          <li
            v-for="(item, index) in suggestions"
            :id="`${listId}-${index}`"
            :key="item.value"
            role="option"
            :aria-selected="index === active"
            class="flex min-h-9 cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-[length:var(--ui-small)] hover:bg-accent aria-selected:bg-accent"
            @pointerdown.prevent
            @mousemove="active = index"
            @click="choose(item)"
          >
            <span
              v-if="item.emoji"
              class="flex w-6 shrink-0 justify-center text-xl"
              ><CustomEmoji
                v-if="item.emoji.url && !item.emoji.unicode"
                :item="item.emoji"
                icon
              /><template v-else>{{ item.value }}</template></span
            >
            <span>{{ item.label }}</span
            ><span
              v-if="item.detail && item.detail !== item.label"
              class="truncate text-muted-foreground"
              >{{ item.detail }}</span
            >
          </li>
        </ul>
      </div>
      <p
        v-else-if="compact && field"
        class="m-0 rounded-t-md bg-card px-3 py-1.5 text-[length:var(--ui-small)] text-muted-foreground shadow-popover"
        :data-status-hint="name === 'status' ? '' : undefined"
      >
        {{ t(field.hint) }}
        <span class="text-faint">{{
          t(
            field.required
              ? 'workspaceUi.commandEditor.required'
              : 'workspaceUi.commandEditor.optional',
          )
        }}</span>
      </p>
    </div>
    <form
      :class="
        compact
          ? 'flex h-[max(calc(48px*var(--interface-scale,1)),calc(30px*var(--interface-scale,1)+18px))] min-w-0 items-center gap-1 rounded-[14px] bg-input px-2 py-0.5 shadow-[inset_0_0_0_1px_var(--border),0_8px_24px_rgb(0_0_0/20%)] max-[700px]:h-10 [@media(pointer:coarse)]:h-[max(calc(48px*var(--interface-scale,1)),calc(30px*var(--interface-scale,1)+18px),62px)] max-[700px]:[@media(pointer:coarse)]:h-[max(calc(40px*var(--interface-scale,1)),calc(44px+4px*var(--interface-scale,1)))]'
          : 'grid gap-2.5 rounded-md bg-card p-3 shadow-popover'
      "
      @submit.prevent="emit('submit', { ...values })"
    >
      <div
        v-if="!compact"
        class="flex items-center justify-between gap-2"
      >
        <strong>{{ title ?? `/${name}` }}</strong
        ><button
          type="button"
          class="rounded p-1 text-muted-foreground hover:bg-accent"
          :aria-label="t('common.cancel')"
          @click="emit('cancel')"
        >
          <AppIcon
            name="close"
            :size="16"
          />
        </button>
      </div>
      <span
        v-if="compact"
        class="shrink-0 rounded-md bg-secondary px-2 py-1.5 font-code text-[length:var(--ui-small)] font-medium text-primary"
        >/{{ name }}</span
      >
      <div
        :class="
          compact
            ? 'flex h-full min-w-0 flex-1 items-center gap-1 overflow-x-auto overflow-y-hidden [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'
            : 'grid gap-2'
        "
      >
        <label
          v-for="(item, index) in fields"
          :key="item.name"
          :class="
            compact
              ? 'flex min-w-32 flex-[1_0_8rem] items-center text-[length:var(--ui-caption)] text-muted-foreground'
              : 'grid gap-1 text-[length:var(--ui-small)] text-muted-foreground'
          "
        >
          <span :class="{ 'sr-only': compact }"
            >{{ t(item.label) }}
            <span
              v-if="item.required"
              aria-hidden="true"
              class="text-primary"
              >*</span
            ></span
          >
          <select
            v-if="item.choices?.length"
            v-model="values[item.name]"
            :data-command-argument="item.name"
            :aria-required="item.required || undefined"
            class="min-w-0 w-full rounded-md border border-border bg-background px-2 py-1.5 text-foreground outline-none focus:border-primary [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:text-base"
            @focus="focusField(index)"
          >
            <option
              v-for="choice in item.choices"
              :key="choice.value"
              :value="choice.value"
            >
              {{
                choice.value === 'keep' && keepLabel
                  ? changedKeepLabel && initialChanged(item.name)
                    ? changedKeepLabel
                    : keepLabel
                  : t(choice.label)
              }}
            </option>
          </select>
          <span
            v-else
            class="flex w-full min-w-0 items-center gap-1"
          >
            <span
              v-if="provider(item) === 'emoji' && /^:[\w+-]+:$/.test(values[item.name] ?? '')"
              :data-status-emoji-preview="name === 'status' ? '' : undefined"
              ><StatusEmoji :value="values[item.name] ?? ''"
            /></span>
            <input
              v-model="values[item.name]"
              :data-command-argument="item.name"
              :type="item.type === 'secret' ? 'password' : 'text'"
              :autocomplete="item.type === 'secret' ? 'new-password' : 'off'"
              :aria-label="t(item.label)"
              :aria-required="item.required || undefined"
              :placeholder="
                item.placeholder ? t(item.placeholder) : compact ? t(item.label) : undefined
              "
              :role="provider(item) ? 'combobox' : undefined"
              :aria-expanded="provider(item) ? suggestionsOpen && focused === index : undefined"
              :aria-controls="suggestionsOpen && focused === index ? listId : undefined"
              :aria-activedescendant="
                suggestionsOpen && focused === index ? `${listId}-${active}` : undefined
              "
              class="min-w-0 w-full rounded-md border border-border bg-background px-2 py-1.5 text-foreground outline-none focus:border-primary [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:text-base"
              @focus="focusField(index)"
              @blur="searching = false"
              @input="inputField"
            />
          </span>
        </label>
      </div>
      <p
        v-if="error && !compact"
        role="alert"
        class="m-0 w-full text-[length:var(--ui-small)] text-destructive"
      >
        {{ error }}
      </p>
      <div
        :class="
          compact
            ? 'flex shrink-0 items-center gap-1'
            : 'flex flex-wrap items-center justify-end gap-2'
        "
      >
        <slot name="actions" /><UiButton
          type="button"
          variant="ghost"
          :size="compact ? 'icon-sm' : 'sm'"
          :aria-label="compact ? t('common.cancel') : undefined"
          :title="compact ? t('common.cancel') : undefined"
          :disabled="busy"
          @click="emit('cancel')"
          ><AppIcon
            v-if="compact"
            name="close"
            :size="16"
          /><template v-else>{{ t('common.cancel') }}</template></UiButton
        ><UiButton
          type="submit"
          size="sm"
          :disabled="busy"
          >{{
            busy
              ? (busyLabel ?? t('workspaceUi.commandEditor.running'))
              : (submitLabel ?? t('workspaceUi.commandEditor.run'))
          }}</UiButton
        >
      </div>
    </form>
  </section>
</template>
