<template>
  <div
    v-if="isOpen"
    ref="menuEl"
    class="context-menu"
    :style="menuStyle"
  >
    <button
      v-for="item in items"
      :key="item.id"
      type="button"
      class="context-item"
      :class="{ danger: item.danger }"
      @click="onSelect(item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
interface ContextMenuItem {
  id: string
  label: string
  danger?: boolean
}

interface Props {
  isOpen: boolean
  x: number
  y: number
  items: ContextMenuItem[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  select: [id: string]
}>()

const menuEl = ref<HTMLDivElement>()

const menuStyle = computed(() => ({
  left: `${props.x}px`,
  top: `${props.y}px`,
}))

function onSelect(id: string) {
  emit('select', id)
  emit('close')
}

function onGlobalPointerDown(event: MouseEvent) {
  if (!props.isOpen) return
  if (!menuEl.value) return
  if (menuEl.value.contains(event.target as Node)) return
  emit('close')
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (!props.isOpen) return
  if (event.key !== 'Escape') return
  emit('close')
}

onMounted(() => {
  window.addEventListener('mousedown', onGlobalPointerDown, { capture: true })
  window.addEventListener('keydown', onGlobalKeydown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('mousedown', onGlobalPointerDown, { capture: true })
  window.removeEventListener('keydown', onGlobalKeydown, { capture: true })
})
</script>

<style scoped>
.context-menu {
  position: fixed;
  min-width: 186px;
  max-width: 240px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(30, 33, 40, 0.98);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.42);
  padding: 6px;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.context-item {
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  text-align: left;
  padding: 7px 8px;
  cursor: pointer;
}

.context-item:hover {
  background: rgba(88, 101, 242, 0.22);
}

.context-item.danger {
  color: #ffb6be;
}

.context-item.danger:hover {
  background: rgba(245, 93, 112, 0.22);
}
</style>
