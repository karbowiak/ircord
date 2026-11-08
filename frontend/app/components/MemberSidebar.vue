<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { Member } from '../../shared/types/chat'
import { parsePrefix } from '../../shared/utils/irc'
import { ariaShortcut, isMac, shortcutLabel } from '../../shared/utils/shortcuts'
const emit = defineEmits<{ profile: [id: string]; close: [] }>()
const chat = useChatStore()
const actions = useWorkspaceActions()
const mac = isMac(navigator.platform)
const menus = useMenus()
const notifications = useNotificationStore()
const nameStyle = useNameStyle()
const filter = ref('')
const scroller = ref<HTMLElement | null>(null)
const network = computed(() => (chat.active ? chat.networkOf(chat.active) : undefined))
const members = computed(() =>
  chat.activeMemberIds.map(chat.member).filter((m): m is Member => !!m),
)
const names: Record<string, string> = {
  '~': 'Owners',
  '&': 'Admins',
  '@': 'Operators',
  '%': 'Half-operators',
  '+': 'Voiced',
}

type Row =
  | { kind: 'group'; key: string; label: string; count: number }
  | { kind: 'member'; key: string; member: Member; prefix: string }
/**
 * Flat rows (group headings and people) for the virtual list, grouped by the highest channel
 * prefix from the network's PREFIX, then presence. Large channels only render visible rows.
 */
const collator = new Intl.Collator()
const presenceGroups = [
  ['online', 'Online'],
  ['away', 'Away'],
  ['offline', 'Offline'],
] as const
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
    ...[...prefixes].map((p) => [p, names[p] ?? `Prefix ${p}`] as const),
    ...presenceGroups,
  ]
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
watch([filter, () => chat.activeId], regroup, { immediate: true })
onBeforeUnmount(() => clearTimeout(regroupTimer))
const virtualizer = useVirtualizer(
  computed(() => ({
    count: rows.value.length,
    getScrollElement: () => scroller.value,
    estimateSize: (i: number) => (rows.value[i]?.kind === 'group' ? 36 : 44),
    getItemKey: (i: number) => rows.value[i]?.key ?? i,
    overscan: 12,
    paddingEnd: 16,
  })),
)
watch(
  () => chat.activeId,
  () => (filter.value = ''),
)
</script>
<template>
  <aside
    class="member-sidebar"
    aria-label="Members"
  >
    <div class="members-header">
      <button
        class="member-search"
        aria-label="Search messages"
        :aria-keyshortcuts="ariaShortcut('search', mac)"
        @click="actions.search()"
      >
        <span>Search {{ network?.name ?? 'messages' }}</span
        ><kbd aria-hidden="true">{{ shortcutLabel('search', mac) }}</kbd
        ><AppIcon
          name="search"
          :size="15"
        />
      </button>
      <button
        class="icon-button member-close"
        aria-label="Close member list"
        @click="emit('close')"
      >
        <AppIcon
          name="close"
          :size="16"
        />
      </button>
    </div>
    <label
      v-if="members.length > 12"
      class="members-filter"
      ><AppIcon
        name="search"
        :size="14" /><input
        v-model="filter"
        aria-label="Filter members"
        placeholder="Filter members"
    /></label>
    <div
      ref="scroller"
      class="members-scroll"
    >
      <div
        class="virtual-members"
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
            class="member-group-heading"
            :style="{ transform: `translateY(${item.start}px)` }"
          >
            {{ (rows[item.index] as { label: string }).label
            }}<span>{{ (rows[item.index] as { count: number }).count }}</span>
          </h3>
          <button
            v-else-if="rows[item.index]"
            :ref="(el) => el && virtualizer.measureElement(el as HTMLElement)"
            :data-index="item.index"
            class="member-button"
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
              class="member-prefix"
              :title="names[(rows[item.index] as { prefix: string }).prefix]"
              >{{ (rows[item.index] as { prefix: string }).prefix }}</span
            ><span
              class="member-name"
              :style="
                (rows[item.index] as { member: Member }).member.status === 'offline'
                  ? { color: 'var(--faint)' }
                  : nameStyle((rows[item.index] as { member: Member }).member)
              "
              >{{ (rows[item.index] as { member: Member }).member.displayName }}</span
            ><AppIcon
              v-if="notifications.ignored((rows[item.index] as { member: Member }).member.nick)"
              name="muted"
              :size="13"
              class="muted-icon"
              aria-label="Ignored"
            /><span
              v-if="(rows[item.index] as { member: Member }).member.role === 'bot'"
              class="bot-badge"
              >BOT</span
            >
          </button>
        </template>
      </div>
      <p
        v-if="filter && !rows.length"
        class="sidebar-empty"
      >
        Nobody matches “{{ filter }}”.
      </p>
    </div>
    <slot />
  </aside>
</template>
