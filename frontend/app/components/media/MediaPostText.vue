<script setup vapor lang="ts">
/**
 * The words of a social post above its media: where and by whom, and the title. A fixed
 * two-line slot (18px byline, 20px title). Read more opens a dialog so a caption never
 * competes with the media for space in the reserved post layout.
 */
const props = defineProps<{
  provider: string
  name?: string
  handle?: string
  title?: string
  caption?: string
  /** The posted link; the title opens it. */
  url: string
}>()
const { t } = useI18n()
const moreId = useId()
const titleText = ref<HTMLElement | null>(null)
const expanded = ref(false)
useViewHistory(`post-text:${useId()}`, expanded)
const truncated = ref(false)
function measure() {
  const element = titleText.value
  if (element && !expanded.value) truncated.value = element.scrollWidth > element.clientWidth + 1
}
useResizeObserver(titleText, measure)
watch([titleText, () => props.title], measure, { flush: 'post' })
watch(expanded, () => nextTick(measure))
const byline = computed(() =>
  [props.provider, props.handle, props.name].filter(Boolean).join(' · '),
)
const more = computed(() => !!props.caption || truncated.value)
</script>
<template>
  <div
    class="media-post-text relative grid h-10 min-w-0 grid-rows-[18px_20px] gap-0.5 overflow-hidden"
  >
    <p class="m-0 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground">
      {{ byline }}
    </p>
    <div class="flex min-w-0 items-baseline gap-2">
      <a
        v-if="title"
        ref="titleText"
        class="min-w-0 truncate font-semibold leading-5 text-foreground wrap-anywhere hover:underline"
        :href="url"
        :title="title"
        target="_blank"
        rel="noopener noreferrer nofollow"
        >{{ title }}</a
      >
      <UiButton
        v-if="more"
        variant="link"
        size="text"
        class="shrink-0 text-[length:var(--ui-caption)] leading-5"
        :aria-expanded="expanded"
        :aria-controls="moreId"
        @click="expanded = true"
        >{{ t('mediaUi.nativeMedia.readMore') }}</UiButton
      >
    </div>
    <AppDialog
      v-if="expanded"
      :title="title || provider"
      @close="expanded = false"
    >
      <div
        :id="moreId"
        class="grid max-h-[65dvh] gap-3 overflow-y-auto px-5 pb-5"
      >
        <p class="m-0 text-[length:var(--ui-caption)] text-muted-foreground">{{ byline }}</p>
        <p
          v-if="caption"
          class="m-0 whitespace-pre-wrap text-[length:var(--ui-small)] leading-[19px] text-body wrap-anywhere"
        >
          {{ caption }}
        </p>
        <UiButton
          as="a"
          :href="url"
          target="_blank"
          rel="noopener noreferrer nofollow"
          variant="secondary"
          class="justify-self-start"
          >{{ t('mediaUi.linkPreview.openOriginal') }}</UiButton
        >
      </div>
    </AppDialog>
  </div>
</template>
