<script setup lang="ts">
import type { MenuEntry, MenuItem } from '../composables/useContextMenu'
const { state, close } = useContextMenu()
const menu = ref<HTMLElement | null>(null)
const submenu = ref<{ index: number; items: MenuEntry[]; x: number; y: number } | null>(null)
const position = ref({ left: '0px', top: '0px' })
const titleId = useId()

function place() {
  const el = menu.value
  if (!el) return
  const { width, height } = el.getBoundingClientRect()
  position.value = {
    left: `${Math.max(8, Math.min(state.x, window.innerWidth - width - 8))}px`,
    top: `${Math.max(8, state.y + height > window.innerHeight - 8 ? state.y - height : state.y)}px`,
  }
}
watch(
  () => state.open,
  (open) => {
    submenu.value = null
    if (open)
      nextTick(() => {
        place()
        menu.value
          ?.querySelector<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])')
          ?.focus()
      })
  },
)
function run(item: MenuItem, event: Event) {
  if (item.disabled) return
  if (item.children) return openSubmenu(item, event.currentTarget as HTMLElement)
  close()
  item.action?.()
}
function openSubmenu(item: MenuItem, el: HTMLElement) {
  const box = el.getBoundingClientRect()
  const index = Number(el.dataset.index)
  const right = box.right + 200 > window.innerWidth ? box.left - 204 : box.right + 4
  submenu.value = { index, items: item.children ?? [], x: right, y: box.top - 4 }
  nextTick(() => document.querySelector<HTMLElement>('.context-submenu [role="menuitem"]')?.focus())
}
function keys(event: KeyboardEvent) {
  const container = (event.target as HTMLElement).closest('[role="menu"]') as HTMLElement | null
  const items = [...(container?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
  const index = items.indexOf(event.target as HTMLElement)
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    items[(index + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length]?.focus()
  } else if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault()
    items[event.key === 'Home' ? 0 : items.length - 1]?.focus()
  } else if (
    event.key === 'ArrowRight' &&
    (event.target as HTMLElement).getAttribute('aria-haspopup')
  ) {
    event.preventDefault()
    ;(event.target as HTMLElement).click()
  } else if (event.key === 'ArrowLeft' && container?.classList.contains('context-submenu')) {
    event.preventDefault()
    const parent = submenu.value?.index
    submenu.value = null
    nextTick(() => menu.value?.querySelector<HTMLElement>(`[data-index="${parent}"]`)?.focus())
  } else if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    close()
  } else if (event.key === 'Tab') close()
}
function outside(event: PointerEvent) {
  if (state.open && !(event.target as HTMLElement).closest('.context-menu')) close(false)
}
onMounted(() => {
  document.addEventListener('pointerdown', outside, true)
  window.addEventListener('blur', () => close(false))
  window.addEventListener('resize', () => close(false))
})
onBeforeUnmount(() => document.removeEventListener('pointerdown', outside, true))
</script>
<template>
  <Teleport to="body">
    <div
      v-if="state.open"
      ref="menu"
      class="context-menu"
      role="menu"
      :aria-labelledby="state.title ? titleId : undefined"
      :aria-label="state.title ? undefined : 'Actions'"
      :style="position"
      @keydown="keys"
      @contextmenu.prevent
    >
      <p
        v-if="state.title"
        :id="titleId"
        class="context-title"
      >
        {{ state.title }}
      </p>
      <div
        v-if="state.quick.length"
        class="context-quick"
      >
        <button
          v-for="quick in state.quick"
          :key="quick.emoji"
          role="menuitem"
          tabindex="-1"
          :aria-label="quick.label"
          :title="quick.label"
          @click="(close(), quick.action())"
        >
          {{ quick.emoji }}
        </button>
      </div>
      <template
        v-for="(item, index) in state.items"
        :key="index"
      >
        <div
          v-if="item === '-'"
          class="context-separator"
          role="separator"
        />
        <button
          v-else
          role="menuitem"
          :data-index="index"
          :class="{ danger: item.danger, open: submenu?.index === index }"
          :aria-disabled="item.disabled || undefined"
          :aria-haspopup="item.children ? 'menu' : undefined"
          :aria-expanded="item.children ? submenu?.index === index : undefined"
          tabindex="-1"
          @click="run(item, $event)"
          @pointerenter="
            item.children
              ? openSubmenu(item, $event.currentTarget as HTMLElement)
              : (submenu = null)
          "
        >
          <AppIcon
            v-if="item.icon"
            :name="item.icon"
            :size="15"
          /><span class="context-label">{{ item.label }}</span
          ><span
            v-if="item.hint"
            class="context-hint"
            >{{ item.hint }}</span
          ><AppIcon
            v-if="item.children"
            name="right"
            :size="14"
          /><AppIcon
            v-else-if="item.checked"
            name="check"
            :size="14"
          />
        </button>
      </template>
    </div>
    <div
      v-if="state.open && submenu"
      class="context-menu context-submenu"
      role="menu"
      :style="{ left: `${submenu.x}px`, top: `${submenu.y}px` }"
      @keydown="keys"
      @contextmenu.prevent
    >
      <template
        v-for="(item, index) in submenu.items"
        :key="index"
      >
        <div
          v-if="item === '-'"
          class="context-separator"
          role="separator"
        />
        <button
          v-else
          role="menuitem"
          :class="{ danger: item.danger }"
          :aria-disabled="item.disabled || undefined"
          tabindex="-1"
          @click="run(item, $event)"
        >
          <AppIcon
            v-if="item.icon"
            :name="item.icon"
            :size="15"
          /><span class="context-label">{{ item.label }}</span
          ><AppIcon
            v-if="item.checked"
            name="check"
            :size="14"
          />
        </button>
      </template>
    </div>
  </Teleport>
</template>
