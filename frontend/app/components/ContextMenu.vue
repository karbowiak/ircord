<script setup lang="ts">
import type { MenuItem } from '../composables/useContextMenu'
const { state, close } = useContextMenu()
const chat = useChatStore()
// Menus describe current privileges. Close an open menu when the selected
// network or channel permissions change instead of leaving stale actions live.
watch(
  () => [chat.network, chat.active?.joined, chat.active?.memberModes, chat.active?.modes],
  () => {
    if (state.open) close(false)
  },
)
// The menu opens at a point (the pointer, or below a button): a zero-size anchor sits there.
const titleId = useId()
const anchor = computed(() => ({ left: `${state.x}px`, top: `${state.y}px` }))
// A press outside closes without pulling focus back; the press goes on to what it hit.
let outside = false
function openChange(open: boolean) {
  if (open) return
  close(!outside)
  outside = false
}
/** Like the menu's earlier behaviour: the first available item takes focus on open. */
function focusFirst(event: Event) {
  event.preventDefault()
  ;(event.target as HTMLElement | null)
    ?.querySelector<HTMLElement>('[role="menuitem"]:not([data-disabled])')
    ?.focus()
}
/** Closes (returning focus) before acting, so a dialog the action opens starts from the opener. */
function run(item: MenuItem | { action: () => void }, event: Event) {
  event.preventDefault()
  close()
  item.action?.()
}
const dismiss = () => close(false)
onMounted(() => {
  window.addEventListener('blur', dismiss)
  window.addEventListener('resize', dismiss)
})
onBeforeUnmount(() => {
  window.removeEventListener('blur', dismiss)
  window.removeEventListener('resize', dismiss)
})
</script>
<template>
  <UiDropdownMenu
    :open="state.open"
    :modal="false"
    @update:open="openChange"
  >
    <Teleport to="body">
      <UiDropdownMenuTrigger as-child>
        <span
          class="context-menu-anchor pointer-events-none fixed size-0"
          :style="anchor"
          aria-hidden="true"
        />
      </UiDropdownMenuTrigger>
    </Teleport>
    <UiDropdownMenuContent
      class="context-menu"
      side="bottom"
      align="start"
      :side-offset="0"
      :aria-labelledby="state.title ? titleId : undefined"
      :aria-label="state.title ? undefined : 'Actions'"
      @open-auto-focus="focusFirst"
      @close-auto-focus.prevent
      @pointer-down-outside="outside = true"
      @focus-outside="outside = true"
      @contextmenu.prevent
    >
      <UiDropdownMenuLabel
        v-if="state.title"
        :id="titleId"
        class="context-title"
      >
        {{ state.title }}
      </UiDropdownMenuLabel>
      <div
        v-if="state.quick.length"
        class="context-quick flex gap-1 pt-0.5 pr-0.5 pb-1.5 pl-0.5 mb-1 border-b [border-bottom-style:solid] border-b-border"
      >
        <UiDropdownMenuItem
          v-for="quick in state.quick"
          :key="quick.emoji"
          variant="quick"
          :aria-label="quick.label"
          :title="quick.label"
          @select="run(quick, $event)"
        >
          <CustomEmoji
            v-if="quick.item?.url"
            :item="quick.item"
            icon
          /><template v-else>{{ quick.emoji }}</template>
        </UiDropdownMenuItem>
      </div>
      <template
        v-for="(item, index) in state.items"
        :key="index"
      >
        <UiDropdownMenuSeparator
          v-if="item === '-'"
          class="context-separator"
        />
        <UiDropdownMenuSub v-else-if="item.children">
          <UiDropdownMenuSubTrigger
            :disabled="item.disabled"
            :title="item.title ?? item.hint"
          >
            <AppIcon
              v-if="item.icon"
              :name="item.icon"
              :size="15"
            /><span
              class="context-label flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
              >{{ item.label }}</span
            ><span
              v-if="item.hint"
              class="context-hint text-[11px] opacity-70"
              >{{ item.hint }}</span
            ><AppIcon
              name="right"
              :size="14"
            />
          </UiDropdownMenuSubTrigger>
          <UiDropdownMenuSubContent
            class="context-menu context-submenu"
            @contextmenu.prevent
          >
            <template
              v-for="(child, childIndex) in item.children"
              :key="childIndex"
            >
              <UiDropdownMenuSeparator
                v-if="child === '-'"
                class="context-separator"
              />
              <UiDropdownMenuItem
                v-else
                :variant="child.danger ? 'destructive' : 'default'"
                :disabled="child.disabled"
                @select="run(child, $event)"
              >
                <AppIcon
                  v-if="child.icon"
                  :name="child.icon"
                  :size="15"
                /><span
                  class="context-label flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
                  >{{ child.label }}</span
                ><AppIcon
                  v-if="child.checked"
                  name="check"
                  :size="14"
                />
              </UiDropdownMenuItem>
            </template>
          </UiDropdownMenuSubContent>
        </UiDropdownMenuSub>
        <UiDropdownMenuItem
          v-else
          :variant="item.danger ? 'destructive' : 'default'"
          :disabled="item.disabled"
          :title="item.title ?? item.hint"
          @select="run(item, $event)"
        >
          <AppIcon
            v-if="item.icon"
            :name="item.icon"
            :size="15"
          /><span
            class="context-label flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
            >{{ item.label }}</span
          ><span
            v-if="item.hint"
            class="context-hint text-[11px] opacity-70"
            >{{ item.hint }}</span
          ><AppIcon
            v-if="item.checked"
            name="check"
            :size="14"
          />
        </UiDropdownMenuItem>
      </template>
    </UiDropdownMenuContent>
  </UiDropdownMenu>
</template>
