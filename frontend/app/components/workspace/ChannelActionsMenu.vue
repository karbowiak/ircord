<script setup lang="ts">
const emit = defineEmits<{ join: [] }>()
const { t } = useI18n()
const actions = useWorkspaceActions()
const open = ref(false)
let hover = false
let timer: ReturnType<typeof setTimeout> | undefined
function enter(event: PointerEvent) {
  if (event.pointerType !== 'mouse') return
  clearTimeout(timer)
  if (!open.value)
    timer = setTimeout(() => {
      hover = true
      open.value = true
    }, 250)
}
function leave(event: PointerEvent) {
  if (event.pointerType !== 'mouse') return
  clearTimeout(timer)
  if (hover)
    timer = setTimeout(() => {
      open.value = false
      hover = false
    }, 350)
}
function change(value: boolean) {
  open.value = value
  if (!value) hover = false
}
onBeforeUnmount(() => clearTimeout(timer))
</script>
<template>
  <UiDropdownMenu
    :open="open"
    :modal="false"
    @update:open="change"
  >
    <UiDropdownMenuTrigger as-child>
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="size-[22px]"
        :aria-label="t('channelCreate.menu')"
        @pointerenter="enter"
        @pointerleave="leave"
        @pointerdown.capture="hover && $event.preventDefault()"
      >
        <AppIcon
          name="plus"
          :size="14"
        />
      </UiButton>
    </UiDropdownMenuTrigger>
    <UiDropdownMenuContent
      align="end"
      @pointerenter="enter"
      @pointerleave="leave"
      @open-auto-focus="hover && $event.preventDefault()"
    >
      <UiDropdownMenuItem @select="emit('join')">{{
        t('channelCreate.joinExisting')
      }}</UiDropdownMenuItem>
      <UiDropdownMenuItem @select="actions.createChannel()">{{
        t('channelCreate.title')
      }}</UiDropdownMenuItem>
    </UiDropdownMenuContent>
  </UiDropdownMenu>
</template>
