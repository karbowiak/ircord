<script setup lang="ts">
import { overlayOpen } from '../../shared/utils/focus'
defineProps<{ title: string; icon?: string; description?: string; meta?: string }>()
const emit = defineEmits<{ close: [] }>()
const { mobileNav } = useWorkspacePane()
const root = ref<HTMLElement | null>(null)
function focus() {
  nextTick(() =>
    root.value?.querySelector<HTMLElement>('[autofocus]')?.focus({ preventScroll: true }),
  )
}
onMounted(focus)
onActivated(focus)
function escape(event: KeyboardEvent) {
  if (event.key !== 'Escape' || event.defaultPrevented || overlayOpen()) return
  event.preventDefault()
  emit('close')
}
</script>
<template>
  <!-- Same header geometry as a conversation (ChatHeader), so switching pages doesn't jump. -->
  <section
    ref="root"
    class="workspace-page flex h-full min-h-0 min-w-0 flex-1 flex-col"
    :aria-label="title"
    @keydown="escape"
  >
    <header
      class="workspace-page-header flex h-[var(--header-height)] min-w-0 shrink-0 items-center gap-2 px-4 max-[700px]:px-1.5"
    >
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="mobile-menu hidden shrink-0 max-[899px]:inline-flex"
        aria-label="Open conversations"
        @click="mobileNav = true"
        ><AppIcon
          name="menu"
          :size="18"
      /></UiButton>
      <AppIcon
        v-if="icon"
        :name="icon"
        :size="19"
        class="shrink-0 text-faint"
      />
      <h1
        class="m-0 min-w-0 truncate text-[length:var(--ui-channel-title)] leading-[1.2] font-[650] tracking-[-0.01em] [transform:translateY(-0.08em)]"
        :title="title"
      >
        {{ title }}
      </h1>
      <p
        v-if="description"
        class="m-0 min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden"
        :title="description"
      >
        {{ description }}
      </p>
      <div class="ml-auto flex shrink-0 items-center gap-0.5">
        <span
          v-if="meta"
          class="mr-2 whitespace-nowrap text-[length:var(--ui-caption)] text-muted-foreground tabular-nums"
          >{{ meta }}</span
        >
        <slot name="actions" />
        <AppTooltip text="Return to conversation"
          ><UiButton
            variant="ghost"
            size="icon-sm"
            aria-label="Return to conversation"
            @click="emit('close')"
            ><AppIcon
              name="close"
              :size="17" /></UiButton
        ></AppTooltip>
      </div>
    </header>
    <div class="workspace-page-content flex min-h-0 min-w-0 flex-1 flex-col"><slot /></div>
  </section>
</template>
