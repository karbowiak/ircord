<template>
  <div class="user-list">
    <div class="header">
      <span class="title">Users</span>
      <span class="count">{{ appStore.channelMembersList.length }}</span>
    </div>
    <div class="members">
      <template v-if="operators.length">
        <div class="role-header">Operators — {{ operators.length }}</div>
        <div
          v-for="member in operators"
          :key="member.user.id"
          class="member"
        >
          <span class="mode op">@</span>
          <span class="status-dot" :class="member.user.status" />
          <span class="nick" :style="{ color: nickColor(member.user.username) }">{{ member.user.username }}</span>
        </div>
      </template>
      <template v-if="voiced.length">
        <div class="role-header">Voiced — {{ voiced.length }}</div>
        <div
          v-for="member in voiced"
          :key="member.user.id"
          class="member"
        >
          <span class="mode voice">+</span>
          <span class="status-dot" :class="member.user.status" />
          <span class="nick" :style="{ color: nickColor(member.user.username) }">{{ member.user.username }}</span>
        </div>
      </template>
      <template v-if="regulars.length">
        <div class="role-header">Users — {{ regulars.length }}</div>
        <div
          v-for="member in regulars"
          :key="member.user.id"
          class="member"
        >
          <span class="mode">&nbsp;</span>
          <span class="status-dot" :class="member.user.status" />
          <span class="nick" :style="{ color: nickColor(member.user.username) }">{{ member.user.username }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
const appStore = useAppStore()

const operators = computed(() =>
  appStore.channelMembersList.filter(m => m.mode === 'op')
)
const voiced = computed(() =>
  appStore.channelMembersList.filter(m => m.mode === 'voice')
)
const regulars = computed(() =>
  appStore.channelMembersList.filter(m => m.mode === 'regular')
)

function nickColor(username: string): string {
  const colors = ['#e06c75', '#61afef', '#c678dd', '#e5c07b', '#56b6c2', '#98c379']
  let hash = 0
  for (let i = 0; i < username.length; i++) {
    hash = username.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}
</script>

<style scoped>
.user-list {
  width: 240px;
  height: 100%;
  background-color: var(--bg-dark);
  display: flex;
  flex-direction: column;
  border-left: 1px solid var(--border-dark);
}

.header {
  height: 48px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid var(--border-dark);
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
}

.title {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.count {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

.members {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.role-header {
  padding: 16px 16px 4px 16px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.role-header:first-child {
  padding-top: 8px;
}

.member {
  display: flex;
  align-items: center;
  padding: 4px 16px;
  cursor: pointer;
  transition: background-color 0.1s ease;
}

.member:hover {
  background-color: var(--bg-hover);
}

.mode {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 700;
  width: 16px;
  flex-shrink: 0;
  color: var(--text-muted);
}

.mode.op {
  color: var(--accent-orange);
}

.mode.voice {
  color: var(--accent-green);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 8px;
  flex-shrink: 0;
}

.status-dot.online {
  background-color: var(--accent-green);
}

.status-dot.away {
  background-color: var(--accent-orange);
}

.status-dot.offline {
  background-color: var(--text-faint);
}

.nick {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
