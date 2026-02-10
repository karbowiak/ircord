<template>
  <div class="app-layout">
    <ServerSidebar />
    <ChannelSidebar />
    <MainChat />
  </div>
</template>

<script setup lang="ts">
const appStore = useAppStore()
const router = useRouter()
const route = useRoute()

type NavigationState = {
  serverId: string | null
  channelId: string | null
}

const STORAGE_KEY = 'ircord-navigation-state'
const isApplyingRoute = ref(false)
const isHydrated = ref(false)

function normalizeQueryValue(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null
}

function parseRouteState(): NavigationState {
  return {
    serverId: normalizeQueryValue(route.query.s),
    channelId: normalizeQueryValue(route.query.c),
  }
}

function currentState(): NavigationState {
  return {
    serverId: appStore.activeServerId,
    channelId: appStore.activeChannelId,
  }
}

function toQuery(state: NavigationState): Record<string, string> {
  const query: Record<string, string> = {}

  if (state.serverId) query.s = state.serverId
  if (state.channelId) query.c = state.channelId

  return query
}

function sameState(a: NavigationState, b: NavigationState): boolean {
  return a.serverId === b.serverId && a.channelId === b.channelId
}

function sameQueryShape(query: Record<string, string>): boolean {
  const active = parseRouteState()
  return active.serverId === (query.s ?? null) && active.channelId === (query.c ?? null)
}

function saveState(state: NavigationState) {
  if (!import.meta.client) return
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

function loadStoredState(): NavigationState | null {
  if (!import.meta.client) return null

  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return null

  try {
    const parsed = JSON.parse(raw) as { serverId?: unknown, channelId?: unknown }
    return {
      serverId: normalizeQueryValue(parsed.serverId),
      channelId: normalizeQueryValue(parsed.channelId),
    }
  } catch {
    return null
  }
}

watch(
  () => route.query,
  () => {
    if (!isHydrated.value) return

    const next = parseRouteState()
    const current = currentState()
    if (sameState(next, current)) return

    isApplyingRoute.value = true
    appStore.applyNavigationState(next.serverId, next.channelId)
    saveState(currentState())

    nextTick(() => {
      isApplyingRoute.value = false
    })
  }
)

watch(
  () => [appStore.activeServerId, appStore.activeChannelId],
  () => {
    if (!isHydrated.value) return

    const state = currentState()
    saveState(state)

    if (isApplyingRoute.value) return

    const query = toQuery(state)
    if (sameQueryShape(query)) return

    router.push({ query })
  }
)

onMounted(async () => {
  await appStore.initializeIrcConnections()

  const routeState = parseRouteState()
  const hasRouteState = Boolean(routeState.serverId || routeState.channelId)
  const initialState = hasRouteState ? routeState : (loadStoredState() ?? routeState)

  appStore.applyNavigationState(initialState.serverId, initialState.channelId)

  const normalizedQuery = toQuery(currentState())
  if (!sameQueryShape(normalizedQuery)) {
    router.replace({ query: normalizedQuery })
  }

  isHydrated.value = true
})
</script>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
}
</style>
