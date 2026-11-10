<script setup vapor lang="ts">
import type { NoteBlock } from '../../../shared/utils/changelog'

/**
 * Release-note blocks from parseNotes, rendered with text bindings only. Note headings sit below
 * the release's own heading (`depth`), so a `##` in the notes never outranks the version title.
 */
const props = withDefaults(defineProps<{ blocks: NoteBlock[]; depth?: number }>(), { depth: 4 })
const headingTag = (level: number) => `h${Math.min(6, props.depth + level - 1)}`
</script>
<template>
  <template
    v-for="(block, index) in blocks"
    :key="index"
  >
    <component
      :is="headingTag(block.level)"
      v-if="block.type === 'heading'"
      class="note-heading"
      ><NoteInline :content="block.content"
    /></component>
    <p v-else-if="block.type === 'paragraph'"><NoteInline :content="block.content" /></p>
    <blockquote v-else-if="block.type === 'quote'">
      <NoteInline :content="block.content" />
    </blockquote>
    <pre v-else-if="block.type === 'code'"><code>{{ block.text }}</code></pre>
    <hr v-else-if="block.type === 'rule'" >
    <component
      :is="block.ordered ? 'ol' : 'ul'"
      v-else-if="block.type === 'list'"
    >
      <li
        v-for="(item, itemIndex) in block.items"
        :key="itemIndex"
      >
        <NoteInline :content="item.content" /><NoteBlocks
          v-if="item.children"
          :blocks="[item.children]"
          :depth="depth"
        />
      </li>
    </component>
  </template>
</template>
