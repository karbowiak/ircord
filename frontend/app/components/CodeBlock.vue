<script setup lang="ts">
import { cachedHighlight, highlightCode } from '../../shared/utils/highlight'
/** A fenced code block: highlighted when its fence names a known language, with a copy button. */
const props = defineProps<{ code: string; lang?: string }>()
const chat = useChatStore()

const html = ref<string | null>(null)
watch(
  () => [props.code, props.lang] as const,
  ([code, lang], _, onCleanup) => {
    let current = true
    onCleanup(() => (current = false))
    html.value = cachedHighlight(code, lang) ?? null
    if (html.value !== null) return
    // Until (or unless) the grammar loads, the block shows the same text unhighlighted.
    void highlightCode(code, lang)
      .then((value) => {
        if (current) html.value = value
      })
      .catch(() => {})
  },
  { immediate: true },
)

const copied = ref(false)
let reset: ReturnType<typeof setTimeout> | undefined
/** Copies the source exactly as shown: no highlighting, label or fences. */
async function copy() {
  try {
    await navigator.clipboard.writeText(props.code)
  } catch {
    chat.notify('Couldn’t copy to the clipboard.')
    return
  }
  copied.value = true
  clearTimeout(reset)
  reset = setTimeout(() => (copied.value = false), 1500)
}
onBeforeUnmount(() => clearTimeout(reset))
</script>
<template>
  <div class="code-frame">
    <pre
      class="code-block"
      tabindex="0"
      role="group"
      :aria-label="lang ? `${lang} code` : 'Code'"
    ><span
      v-if="lang"
      class="code-lang"
      aria-hidden="true"
    >{{ lang }}</span><code
      v-if="html !== null"
      class="is-highlighted"
      v-html="html"
    /><code v-else>{{ code }}</code></pre>
    <button
      type="button"
      class="code-copy"
      :class="{ 'is-copied': copied }"
      :aria-label="copied ? 'Copied' : 'Copy code'"
      :title="copied ? 'Copied' : 'Copy code'"
      @click.stop="copy"
    >
      <AppIcon
        :name="copied ? 'check' : 'copy'"
        :size="14"
      />
    </button>
    <span
      class="sr-only"
      aria-live="polite"
      >{{ copied ? 'Code copied' : '' }}</span
    >
  </div>
</template>
