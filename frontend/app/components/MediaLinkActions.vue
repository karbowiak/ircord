<script setup lang="ts">
import { publicMediaURL } from '../../shared/utils/link-media'

const props = defineProps<{ url: string; alwaysVisible?: boolean; openLabel?: string }>()
const { t } = useI18n()
const chat = useChatStore()
const href = computed(() => publicMediaURL(props.url))

async function copy() {
  try {
    await navigator.clipboard.writeText(props.url)
    chat.notify(t('mediaUi.linkActions.copySuccess'))
  } catch {
    chat.notify(t('mediaUi.linkActions.copyBlocked'))
  }
}
</script>
<template>
  <div
    v-if="href"
    class="media-link-actions flex gap-0.5 rounded-md bg-background/95 p-0.5 text-foreground shadow-control transition-opacity group-hover/media:opacity-100 group-hover/media:pointer-events-auto group-focus-within/media:opacity-100 group-focus-within/media:pointer-events-auto focus-within:opacity-100 focus-within:pointer-events-auto [@media(hover:none)]:opacity-100 [@media(hover:none)]:pointer-events-auto [@media(any-pointer:coarse)]:opacity-100 [@media(any-pointer:coarse)]:pointer-events-auto motion-reduce:transition-none"
    :class="alwaysVisible ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'"
    @click.stop
    @dblclick.stop
    @pointerdown.stop
    @pointerup.stop
    @contextmenu.stop
  >
    <AppTooltip :text="t('mediaUi.linkActions.copy')">
      <UiButton
        type="button"
        variant="ghost"
        size="icon-sm"
        :aria-label="t('mediaUi.linkActions.copyNamed', { url })"
        @click="copy"
      >
        <AppIcon
          name="copy"
          :size="15"
        />
      </UiButton>
    </AppTooltip>
    <AppTooltip :text="openLabel || t('mediaUi.linkActions.open')">
      <UiButton
        as="a"
        variant="ghost"
        size="icon-sm"
        :href="href"
        target="_blank"
        rel="noopener noreferrer nofollow"
        :aria-label="openLabel || t('mediaUi.linkActions.openNamed', { url })"
      >
        <AppIcon
          name="external"
          :size="15"
        />
      </UiButton>
    </AppTooltip>
  </div>
</template>
