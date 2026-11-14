<!-- Not Vapor: Reka's `as-child` trigger merges its props into a VNode slot. -->
<script setup lang="ts">
import { DropdownMenuCheckboxItem } from 'reka-ui'
import { dropdownMenuItemVariants } from '../ui/dropdown-menu'
import type { MenuItem } from '../../composables/shared/useContextMenu'
const { t: $t } = useI18n()
const { state, close } = useContextMenu()
const chat = useChatStore()
// Menus describe current privileges. Close an open menu when the selected
// network or channel permissions change instead of leaving stale actions live.
watch(
  () => [
    chat.activeId,
    chat.network,
    chat.active?.joined,
    chat.active?.memberModes,
    chat.active?.modes,
  ],
  () => {
    if (state.open) close(false)
  },
)
// The menu opens at a point (the pointer, or below a button): a zero-size anchor sits there.
const titleId = useId()
const menuId = useId()
const anchor = computed(() => ({ left: `${state.x}px`, top: `${state.y}px` }))
// A press outside closes without pulling focus back; the press goes on to what it hit.
let outside = false
function openChange(open: boolean) {
  if (open) return
  close(!outside)
  outside = false
}
const items = ':is([role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"])'
/** Like the menu's earlier behaviour: the first available item takes focus on open. */
function focusFirst(event: Event) {
  event.preventDefault()
  const menu = event.target as HTMLElement | null
  ;(
    menu?.querySelector<HTMLElement>(`${items}:not([data-disabled], [aria-disabled="true"])`) ??
    menu?.querySelector<HTMLElement>(`${items}:not([data-disabled])`)
  )?.focus()
}
/**
 * An unavailable item with a reason (`title`) stays focusable, so keyboard and screen-reader users
 * reach the reason as its description; choosing it does nothing. Without a reason it is disabled.
 */
const explained = (item: MenuItem) => !!item.disabled && !!item.title
const reasonId = (index: number | string) => `${menuId}-reason-${index}`
/** Closes (returning focus) before acting, so a dialog the action opens starts from the opener. */
function run(item: MenuItem | { action: () => void }, event: Event) {
  event.preventDefault()
  if ('disabled' in item && item.disabled) return
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
      :aria-label="state.title ? undefined : $t('workspaceUi.shell.actions')"
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
              <!-- Choices (a notification level) expose their checked state. -->
              <DropdownMenuCheckboxItem
                v-else-if="child.checked !== undefined"
                :model-value="child.checked"
                :disabled="child.disabled"
                :class="dropdownMenuItemVariants({ variant: 'default' })"
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
              </DropdownMenuCheckboxItem>
              <UiDropdownMenuItem
                v-else
                :variant="child.danger ? 'destructive' : 'default'"
                :disabled="child.disabled && !explained(child)"
                :aria-disabled="explained(child) || undefined"
                :aria-describedby="
                  explained(child) ? reasonId(`${index}-${childIndex}`) : undefined
                "
                :class="{ 'opacity-45': explained(child) }"
                :title="child.title"
                @select="run(child, $event)"
              >
                <AppIcon
                  v-if="child.icon"
                  :name="child.icon"
                  :size="15"
                /><span
                  class="context-label flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap"
                  >{{ child.label }}</span
                ><span
                  v-if="explained(child)"
                  :id="reasonId(`${index}-${childIndex}`)"
                  hidden
                  >{{ child.title }}</span
                >
              </UiDropdownMenuItem>
            </template>
          </UiDropdownMenuSubContent>
        </UiDropdownMenuSub>
        <DropdownMenuCheckboxItem
          v-else-if="item.checked !== undefined"
          :model-value="item.checked"
          :disabled="item.disabled"
          :class="dropdownMenuItemVariants({ variant: 'default' })"
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
          ><AppIcon
            v-if="item.checked"
            name="check"
            :size="14"
          />
        </DropdownMenuCheckboxItem>
        <UiDropdownMenuItem
          v-else
          :variant="item.danger ? 'destructive' : 'default'"
          :disabled="item.disabled && !explained(item)"
          :aria-disabled="explained(item) || undefined"
          :aria-describedby="explained(item) ? reasonId(index) : undefined"
          :class="{ 'opacity-45': explained(item) }"
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
          ><span
            v-if="explained(item)"
            :id="reasonId(index)"
            hidden
            >{{ item.title }}</span
          >
        </UiDropdownMenuItem>
      </template>
    </UiDropdownMenuContent>
  </UiDropdownMenu>
</template>
