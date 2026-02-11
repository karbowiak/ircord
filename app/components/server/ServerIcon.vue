<template>
  <div
    class="server-icon"
    :class="{ 'active': isActive }"
    :title="label"
    @click="onClick"
    @contextmenu.prevent="onContextMenu"
  >
    <div class="icon-wrapper">
      <span class="icon" :class="{ 'is-text': !isEmoji }">{{ icon }}</span>
    </div>
    <div class="pill" />
  </div>
</template>

<script setup lang="ts">
interface Props {
  icon: string
  label?: string
  isActive?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  isActive: false,
  label: '',
})

const emit = defineEmits<{
  click: []
  contextmenu: [event: MouseEvent]
}>()

const isEmoji = computed(() => {
  // Simple heuristic: if it's 1-3 chars and all non-ASCII, treat as emoji
  return props.icon.length <= 3 && /[^\x00-\x7F]/.test(props.icon)
})

function onClick() {
  emit('click')
}

function onContextMenu(event: MouseEvent) {
  emit('contextmenu', event)
}
</script>

<style scoped>
.server-icon {
  position: relative;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  margin-bottom: 8px;
}

.server-icon:hover .icon-wrapper {
  border-radius: 16px;
  background-color: var(--accent-blurple);
}

.server-icon.active .icon-wrapper {
  border-radius: 16px;
  background-color: var(--accent-blurple);
}

.server-icon:hover .pill {
  height: 20px;
}

.server-icon.active .pill {
  height: 40px;
}

.icon-wrapper {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background-color: var(--bg-primary);
  transition: border-radius 0.2s ease, background-color 0.2s ease;
}

.icon {
  font-size: 24px;
  color: var(--text-body);
  transition: color 0.2s ease;
}

.icon.is-text {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.server-icon:hover .icon,
.server-icon.active .icon {
  color: var(--text-primary);
}

.pill {
  position: absolute;
  left: -16px;
  top: 50%;
  transform: translateY(-50%);
  width: 8px;
  height: 0;
  background-color: white;
  border-radius: 0 4px 4px 0;
  transition: height 0.2s ease;
}
</style>
