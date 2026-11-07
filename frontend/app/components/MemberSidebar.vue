<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { Member } from '../../shared/types/chat'
import { parsePrefix, topPrefix } from '../../shared/utils/irc'
const emit = defineEmits<{ profile: [id: string]; close: [] }>()
const chat = useChatStore()
const menus = useMenus()
const notifications = useNotificationStore()
const nameStyle = useNameStyle()
const filter = ref('')
const scroller = ref<HTMLElement | null>(null)
const network = computed(() => (chat.active ? chat.networkOf(chat.active) : undefined))
const members = computed(
  () => chat.active?.memberIds.map(chat.member).filter((m): m is Member => !!m) ?? [],
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
const rows = computed<Row[]>(() => {
  const { prefixes } = parsePrefix(network.value?.isupport)
  const q = filter.value.trim().toLowerCase()
  const withPrefix = members.value
    .filter((m) => !q || `${m.nick} ${m.displayName}`.toLowerCase().includes(q))
    .map((member) => ({ member, prefix: topPrefix(member.id, chat.active, network.value) }))
  const groups = [
    ...[...prefixes].map((p) => ({
      label: names[p] ?? `Prefix ${p}`,
      test: (x: (typeof withPrefix)[number]) => x.prefix === p,
    })),
    {
      label: 'Online',
      test: (x: (typeof withPrefix)[number]) => !x.prefix && x.member.status === 'online',
    },
    {
      label: 'Away',
      test: (x: (typeof withPrefix)[number]) => !x.prefix && x.member.status === 'away',
    },
    {
      label: 'Offline',
      test: (x: (typeof withPrefix)[number]) => !x.prefix && x.member.status === 'offline',
    },
  ]
  const result: Row[] = []
  for (const group of groups) {
    const list = withPrefix
      .filter(group.test)
      .sort((a, b) => a.member.nick.localeCompare(b.member.nick))
    if (!list.length) continue
    result.push({
      kind: 'group',
      key: `group:${group.label}`,
      label: group.label,
      count: list.length,
    })
    for (const x of list)
      result.push({ kind: 'member', key: x.member.id, member: x.member, prefix: x.prefix })
  }
  return result
})
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
      <span>Members</span><span class="members-count">{{ members.length }}</span>
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
