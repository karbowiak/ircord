<script setup vapor lang="ts">
import type { Inline } from '../../../shared/utils/changelog'

defineProps<{ content: Inline[] }>()
</script>
<template>
  <template
    v-for="(run, index) in content"
    :key="index"
    ><a
      v-if="run.href"
      :href="run.href"
      target="_blank"
      rel="noopener noreferrer"
      >{{ run.text }}</a
    ><code v-else-if="run.code">{{ run.text }}</code
    ><strong v-else-if="run.bold && run.italic"
      ><em>{{ run.text }}</em></strong
    ><strong v-else-if="run.bold">{{ run.text }}</strong
    ><em v-else-if="run.italic">{{ run.text }}</em
    ><template v-else>{{ run.text }}</template></template
  >
</template>
