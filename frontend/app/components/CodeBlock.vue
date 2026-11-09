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
  <div
    class="code-frame block [box-sizing:border-box] max-w-full my-1 mx-0 bg-rail border border-solid border-border rounded-[6px] overflow-hidden"
  >
    <div
      class="code-head flex items-center justify-between gap-2 min-h-6.5 pt-[3px] pr-1 pb-0 pl-2.5"
    >
      <span
        v-if="lang"
        class="code-lang text-faint font-sans text-[11px] lowercase"
        aria-hidden="true"
        >{{ lang }}</span
      >
      <button
        type="button"
        class="code-copy grid [place-items:center] w-6 h-6 ml-auto p-0 border border-solid border-transparent rounded-[5px] [background:none] text-muted-foreground cursor-pointer [opacity:0] [transition:opacity_0.12s_ease]"
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
    </div>
    <pre
      class="code-block block [box-sizing:border-box] max-w-full max-h-[24rem] m-0 pt-0.5 pr-2.5 pb-2 pl-2.5 overflow-auto overscroll-contain font-code text-[0.88em] leading-[1.45] whitespace-pre [overflow-wrap:normal] [tab-size:4]"
      tabindex="0"
      role="group"
      :aria-label="lang ? `${lang} code` : 'Code'"
    ><!-- eslint-disable vue/no-v-html -- highlight.js escapes the source; html is its markup only --><code
      v-if="html !== null"
      class="is-highlighted"
      v-html="html"
    /><!-- eslint-enable vue/no-v-html --><code v-else>{{ code }}</code></pre>
    <span
      class="sr-only"
      aria-live="polite"
      >{{ copied ? 'Code copied' : '' }}</span
    >
  </div>
</template>
