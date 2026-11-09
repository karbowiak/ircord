<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { Member } from '../../shared/types/chat'
import { parsePrefix } from '../../shared/utils/irc'
const emit = defineEmits<{ profile: [id: string]; close: [] }>()
const { t, locale } = useI18n()
const chat = useChatStore()
const actions = useWorkspaceActions()
const shortcutKeys = useShortcuts()
const menus = useMenus()
const notifications = useNotificationStore()
const nameStyle = useNameStyle()
const filter = ref('')
const scroller = ref<HTMLElement | null>(null)
const network = computed(() => (chat.active ? chat.networkOf(chat.active) : undefined))
const members = computed(() =>
  chat.activeMemberIds.map(chat.member).filter((m): m is Member => !!m),
)
const names = computed<Record<string, string>>(() => ({
  '~': t('workspaceUi.members.owners'),
  '&': t('workspaceUi.members.admins'),
  '@': t('workspaceUi.members.operators'),
  '%': t('workspaceUi.members.halfOperators'),
  '+': t('workspaceUi.members.voiced'),
}))

type Row =
  | { kind: 'group'; key: string; label: string; count: number }
  | { kind: 'member'; key: string; member: Member; prefix: string }
/**
 * Flat rows (group headings and people) for the virtual list, grouped by the highest channel
 * prefix from the network's PREFIX, then presence. Large channels only render visible rows.
 */
function group(): Row[] {
  // One pass over the roster: busy channels have thousands of people and this reruns on
  // presence changes, so no per-group filtering and the PREFIX is parsed once.
  const { prefixes } = parsePrefix(network.value?.isupport)
  const modes = chat.active?.memberModes ?? {}
  const q = filter.value.trim().toLowerCase()
  const buckets = new Map<string, { member: Member; prefix: string; name: string }[]>()
  for (const member of members.value) {
    if (q && !`${member.nick} ${member.displayName}`.toLowerCase().includes(q)) continue
    const own = modes[member.id] ?? ''
    let prefix = ''
    if (own)
      for (const char of prefixes)
        if (own.includes(char)) {
          prefix = char
          break
        }
    const key = prefix || member.status
    let list = buckets.get(key)
    if (!list) buckets.set(key, (list = []))
    list.push({ member, prefix, name: member.displayName || member.nick })
  }
  const groups = [
    ...[...prefixes].map(
      (p) => [p, names.value[p] ?? t('workspaceUi.members.prefix', { prefix: p })] as const,
    ),
    ['online', t('workspaceUi.members.online')] as const,
    ['away', t('workspaceUi.members.away')] as const,
    ['offline', t('workspaceUi.members.offline')] as const,
  ]
  const collator = new Intl.Collator(locale.value)
  const result: Row[] = []
  for (const [key, label] of groups) {
    const list = buckets.get(key)
    if (!list?.length) continue
    list.sort((a, b) => collator.compare(a.name, b.name))
    result.push({ kind: 'group', key: `group:${label}`, label, count: list.length })
    for (const x of list)
      result.push({ kind: 'member', key: x.member.id, member: x.member, prefix: x.prefix })
  }
  return result
}
/**
 * Regrouping thousands of people dozens of times a second (presence changes, joins and parts in
 * busy channels) adds nothing a person can see, so roster changes regroup at most every 250ms.
 * Filtering and switching channels regroup immediately.
 */
const rows = shallowRef<Row[]>([])
let regrouped = 0
let regroupTimer: ReturnType<typeof setTimeout> | undefined
function regroup() {
  clearTimeout(regroupTimer)
  regroupTimer = undefined
  regrouped = performance.now()
  rows.value = group()
}
watch(
  () => [members.value, chat.active?.memberModes, network.value?.isupport],
  () => {
    if (regroupTimer) return
    const wait = regrouped + 250 - performance.now()
    if (wait <= 0) regroup()
    else regroupTimer = setTimeout(regroup, wait)
  },
)
watch([filter, () => chat.activeId, locale], regroup, { immediate: true })
onBeforeUnmount(() => clearTimeout(regroupTimer))
const virtualizer = useVirtualizer(
  computed(() => ({
    count: rows.value.length,
    getScrollElement: () => scroller.value,
    estimateSize: (i: number) => (rows.value[i]?.kind === 'group' ? 32 : 36),
    getItemKey: (i: number) => rows.value[i]?.key ?? i,
    overscan: 12,
    paddingEnd: 16,
  })),
)
watch(
  () => chat.activeId,
  () => (filter.value = ''),
)
/** Presence and ignoring, which rows show only as a dot, opacity and strikethrough. */
const memberStates = (member: Member) =>
  [presenceLabel(member, t), notifications.ignored(member.nick) && t('workspaceUi.members.ignored')]
    .filter(Boolean)
    .join(', ')
</script>
<template>
  <aside
    class="member-sidebar relative flex flex-col bg-background min-w-0 min-h-0 max-[1099px]:[&_.resize-handle]:hidden"
    :aria-label="$t('workspaceUi.header.members')"
  >
    <div
      class="members-header hidden h-[var(--header-height)] shrink-0 items-center gap-2 px-2 max-[1099px]:flex"
    >
      <button
        class="member-search flex-1 min-w-0 flex items-center gap-1.5 h-8 py-0 px-2.5 rounded-md bg-field shadow-[inset_0_0_0_1px_var(--border)] text-muted-foreground text-[13px] font-[500] text-left hover:text-foreground"
        :aria-label="$t('workspaceUi.members.searchMessages')"
        :aria-keyshortcuts="shortcutKeys.aria('search')"
        @click="actions.search()"
      >
        <span class="flex-1 min-w-0 truncate">{{
          network?.name
            ? $t('workspaceUi.members.searchNetwork', { network: network.name })
            : $t('workspaceUi.members.searchMessages')
        }}</span
        ><kbd
          class="[font-family:inherit] text-[11px]"
          aria-hidden="true"
          >{{ shortcutKeys.label('search') }}</kbd
        ><AppIcon
          name="search"
          :size="15"
        />
      </button>
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="member-close hidden ml-auto max-[1099px]:flex"
        :aria-label="$t('workspaceUi.members.closeList')"
        @click="emit('close')"
      >
        <AppIcon
          name="close"
          :size="16"
        />
      </UiButton>
    </div>
    <SearchField
      v-if="members.length > 12"
      v-model="filter"
      class="members-filter mx-2 mt-2"
      :aria-label="$t('workspaceUi.members.filter')"
      :placeholder="$t('workspaceUi.members.filter')"
    />
    <div
      ref="scroller"
      class="members-scroll pt-1 pr-2 pb-4 pl-2 overflow-y-auto flex-1 min-h-0 [touch-action:pan-y_pinch-zoom]"
    >
      <div
        class="virtual-members relative w-full"
        :style="{ height: `${virtualizer.getTotalSize()}px` }"
      >
        <template
          v-for="item in virtualizer.getVirtualItems()"
          :key="String(item.key)"
        >
          <h3
            v-if="rows[item.index]?.kind === 'group'"
            :ref="(el) => el && virtualizer.measureElement(el as HTMLElement)"
            :data-index="item.index"
            class="member-group-heading absolute inset-x-0 top-0 m-0 text-[length:var(--ui-caption)] tracking-[0.02em] uppercase font-medium text-muted-foreground pt-3 pr-2 pb-1 pl-2"
            :style="{ transform: `translateY(${item.start}px)` }"
          >
            {{ (rows[item.index] as { label: string }).label
            }}<span class="ml-1.5">{{ (rows[item.index] as { count: number }).count }}</span>
          </h3>
          <button
            v-else-if="rows[item.index]"
            :ref="(el) => el && virtualizer.measureElement(el as HTMLElement)"
            :data-index="item.index"
            class="member-button absolute inset-x-0 top-0 flex items-center gap-2 w-auto m-0 text-left py-0 px-2 rounded-sm min-h-[calc(var(--row-height)_+_4px)] text-[length:var(--ui-text)] hover:bg-accent [&_.presence]:border-background hover:[&_.presence]:border-accent [&.member-offline_.avatar]:opacity-45 [&.is-ignored_.member-name]:line-through[@media(pointer:coarse)]:min-h-12 [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
            :class="{
              'member-offline':
                (rows[item.index] as { member: Member }).member.status === 'offline',
              'is-ignored': notifications.ignored(
                (rows[item.index] as { member: Member }).member.nick,
              ),
            }"
            :style="{ transform: `translateY(${item.start}px)` }"
            @click="emit('profile', (rows[item.index] as { member: Member }).member.id)"
            @contextmenu="menus.member($event, (rows[item.index] as { member: Member }).member)"
          >
            <UserAvatar
              :member="(rows[item.index] as { member: Member }).member"
              size="small"
              presence
            /><span
              v-if="(rows[item.index] as { prefix: string }).prefix"
              class="member-prefix mr-[-6px] font-code font-[700] text-success"
              :title="names[(rows[item.index] as { prefix: string }).prefix]"
              >{{ (rows[item.index] as { prefix: string }).prefix }}</span
            ><span
              class="member-name flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-[500]"
              :style="
                (rows[item.index] as { member: Member }).member.status === 'offline' ||
                notifications.ignored((rows[item.index] as { member: Member }).member.nick)
                  ? { color: 'var(--faint)' }
                  : nameStyle((rows[item.index] as { member: Member }).member)
              "
              >{{ (rows[item.index] as { member: Member }).member.displayName }}</span
            ><AppIcon
              v-if="notifications.ignored((rows[item.index] as { member: Member }).member.nick)"
              name="muted"
              :size="13"
              class="muted-icon text-faint"
            /><span
              v-if="memberStates((rows[item.index] as { member: Member }).member)"
              class="sr-only"
              >, {{ memberStates((rows[item.index] as { member: Member }).member) }}</span
            ><span
              v-if="(rows[item.index] as { member: Member }).member.role === 'bot'"
              class="bot-badge inline-flex items-center h-4 py-0 px-[5px] rounded-[4px] bg-selection text-primary text-[10px] font-[650] tracking-[0.04em] shrink-0"
              >BOT</span
            >
          </button>
        </template>
      </div>
      <p
        v-if="filter && !rows.length"
        class="sidebar-empty text-[length:var(--ui-caption)] text-faint py-1 px-2"
      >
        {{ $t('workspaceUi.members.noMatches', { filter }) }}
      </p>
    </div>
    <slot />
  </aside>
</template>
