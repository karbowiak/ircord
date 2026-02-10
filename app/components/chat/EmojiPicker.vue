<template>
  <div v-if="isOpen" class="emoji-picker" @click.self="emit('close')">
    <div class="panel" role="dialog" aria-label="Emoji picker">
      <div class="header">
        <input
          ref="searchInputEl"
          v-model="query"
          type="text"
          class="search"
          placeholder="Search emojis"
          @keydown.esc.prevent="emit('close')"
        >
        <button type="button" class="close" @click="emit('close')">x</button>
      </div>

      <div class="categories" role="tablist" aria-label="Emoji categories">
        <button
          v-for="category in emojiCategories"
          :key="category.id"
          type="button"
          class="category"
          :class="{ active: activeCategory === category.id }"
          @click="activeCategory = category.id"
        >
          {{ category.label }}
        </button>
      </div>

      <div class="grid-wrap">
        <div class="grid">
          <button
            v-for="emoji in filteredEmojis"
            :key="emoji.shortcode"
            type="button"
            class="emoji-tile"
            :title="`:${emoji.shortcode}:`"
            @click="selectEmoji(emoji.shortcode)"
          >
            <img
              v-if="emoji.imageUrl"
              :src="emoji.imageUrl"
              :alt="emoji.shortcode"
              class="emoji-image"
              loading="lazy"
            >
            <span v-else class="emoji-char">{{ emoji.char }}</span>
            <span class="emoji-label">:{{ emoji.shortcode }}:</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { emojiCategories, emojiDefinitions } from '~/composables/useEmojiData'

interface Props {
  isOpen: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  select: [shortcode: string]
}>()

const query = ref('')
const activeCategory = ref<(typeof emojiCategories)[number]['id']>('people')
const searchInputEl = ref<HTMLInputElement>()

const filteredEmojis = computed(() => {
  const q = query.value.trim().toLowerCase()
  const inCategory = emojiDefinitions.filter((emoji) => emoji.category === activeCategory.value)

  if (!q) return inCategory

  return inCategory.filter((emoji) => {
    return (
      emoji.shortcode.includes(q)
      || emoji.keywords.some((keyword) => keyword.includes(q))
    )
  })
})

watch(
  () => props.isOpen,
  async (open) => {
    if (!open) return
    await nextTick()
    searchInputEl.value?.focus()
    searchInputEl.value?.select()
  }
)

function selectEmoji(shortcode: string) {
  emit('select', `:${shortcode}:`)
}
</script>

<style scoped>
.emoji-picker {
  position: absolute;
  width: min(600px, calc(100vw - 24px));
  right: 16px;
  bottom: 58px;
  z-index: 35;
}

.panel {
  max-height: min(82vh, 700px);
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(44, 47, 55, 0.98), rgba(39, 42, 49, 0.98));
  box-shadow: 0 18px 45px rgba(0, 0, 0, 0.45);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.header {
  display: flex;
  gap: 10px;
  padding: 14px 14px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.search {
  flex: 1;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 11px;
  background-color: var(--bg-input);
  color: var(--text-body);
  padding: 12px 14px;
  font-family: var(--font-mono);
  font-size: 14px;
  outline: none;
}

.search:focus {
  border-color: var(--accent-blurple);
}

.close {
  border: 0;
  border-radius: 9px;
  background-color: var(--bg-input);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 9px 12px;
  cursor: pointer;
}

.close:hover {
  background-color: var(--bg-active);
}

.categories {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  overflow-x: visible;
  padding: 12px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.category {
  border: 0;
  border-radius: 999px;
  background-color: rgba(255, 255, 255, 0.05);
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 11px;
  white-space: normal;
  padding: 7px 10px;
  cursor: pointer;
}

.category.active {
  background-color: rgba(88, 101, 242, 0.2);
  color: var(--text-primary);
}

.grid-wrap {
  max-height: min(56vh, 560px);
  overflow-y: auto;
}

.grid {
  padding: 14px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(152px, 1fr));
  gap: 12px;
}

.emoji-tile {
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  background-color: rgba(255, 255, 255, 0.02);
  color: var(--text-body);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 132px;
  padding: 10px;
  cursor: pointer;
}

.emoji-tile:hover {
  border-color: rgba(88, 101, 242, 0.45);
  background-color: rgba(88, 101, 242, 0.12);
}

.emoji-char {
  font-size: 34px;
  line-height: 1;
}

.emoji-image {
  width: 46px;
  height: 46px;
  object-fit: contain;
}

.emoji-label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

@media (max-width: 900px) {
  .emoji-picker {
    width: calc(100vw - 12px);
    right: 8px;
  }
}
</style>
