<script setup lang="ts">
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
const props = defineProps<{ items: EmojiItem[]; skinTone: string }>()
const emit = defineEmits<{ choose: [item: EmojiItem]; preview: [item: EmojiItem] }>()
const scroller = ref<HTMLElement | null>(null)
const columns = ref(9)
const activeIndex = ref(0)
const appearance = useAppearanceStore()
const options = computed(() => ({
  count: Math.ceil(props.items.length / columns.value),
  getScrollElement: () => scroller.value,
  estimateSize: () => 40,
  overscan: 3,
}))
const virtualizer = useVirtualizer(options)
const rows = computed(() => virtualizer.value.getVirtualItems())
let observer: ResizeObserver | undefined
onMounted(() => {
  observer = new ResizeObserver(
    (entries) =>
      (columns.value = Math.max(4, Math.floor((entries[0]!.contentRect.width - 8) / 40))),
  )
  if (scroller.value) observer.observe(scroller.value)
})
onBeforeUnmount(() => observer?.disconnect())
watch(
  () => props.items,
  () => {
    activeIndex.value = 0
    virtualizer.value.scrollToOffset(0)
  },
)
function focused(item: EmojiItem, index: number) {
  activeIndex.value = index
  emit('preview', item)
}
function move(event: KeyboardEvent, index: number) {
  const offsets: Record<string, number> = {
    ArrowRight: 1,
    ArrowLeft: -1,
    ArrowDown: columns.value,
    ArrowUp: -columns.value,
    Home: -index,
    End: props.items.length - index - 1,
  }
  if (!(event.key in offsets)) return
  event.preventDefault()
  const next = Math.max(0, Math.min(props.items.length - 1, index + offsets[event.key]!))
  activeIndex.value = next
  virtualizer.value.scrollToIndex(Math.floor(next / columns.value), { align: 'auto' })
  nextTick(() =>
    scroller.value?.querySelector<HTMLButtonElement>(`[data-emoji-index="${next}"]`)?.focus(),
  )
}
</script>
<template>
  <div
    ref="scroller"
    class="emoji-grid-scroll"
    role="grid"
    tabindex="0"
    :aria-rowcount="Math.ceil(items.length / columns)"
    :aria-colcount="columns"
    aria-label="Emoji results"
  >
    <div
      class="emoji-virtual-space relative w-full"
      :style="{ height: `${virtualizer.getTotalSize()}px` }"
    >
      <div
        v-for="row in rows"
        :key="String(row.key)"
        role="row"
        class="emoji-grid-row absolute top-0 left-0 w-full grid"
        :style="{
          transform: `translateY(${row.start}px)`,
          gridTemplateColumns: `repeat(${columns}, minmax(0,1fr))`,
        }"
      >
        <div
          v-for="(item, offset) in items.slice(row.index * columns, (row.index + 1) * columns)"
          :key="item.id"
          role="gridcell"
        >
          <button
            class="emoji-choice w-full h-10 flex items-center justify-center rounded-sm text-[26px] leading-[1]"
            :aria-label="`Insert ${item.label}`"
            :title="`:${item.shortcode}:`"
            :tabindex="row.index * columns + offset === activeIndex ? 0 : -1"
            :data-emoji-index="row.index * columns + offset"
            @focus="focused(item, row.index * columns + offset)"
            @pointerenter="emit('preview', item)"
            @keydown="move($event, row.index * columns + offset)"
            @click="emit('choose', item)"
          >
            <span v-if="item.unicode">{{ emojiText(item, skinTone) }}</span
            ><NuxtImg
              v-else
              :src="appearance.animate ? item.url : item.staticUrl || item.url"
              :alt="`:${item.shortcode}:`"
              decoding="async"
              referrerpolicy="no-referrer"
            />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
