<script setup lang="ts">
import type { ScoreEntry } from '../../shared/utils/hash-field-scores'

/** The Hash Field top 10, with this game's entry highlighted when it placed. */
defineProps<{
  items: ScoreEntry[] | null | undefined
  loading: boolean
  /** Rank to highlight; 0 for none. */
  highlight: number
}>()
</script>
<template>
  <section
    class="hash-field-table grid gap-1.5 w-[min(100%,_360px)]"
    aria-labelledby="hash-field-table-title"
  >
    <h5 id="hash-field-table-title">High scores</h5>
    <SettingsDescription v-if="items === null">
      High scores aren’t available right now.
    </SettingsDescription>
    <SettingsDescription v-else-if="!items"> Loading… </SettingsDescription>
    <table
      v-else
      :aria-busy="loading"
    >
      <thead>
        <tr>
          <th scope="col">#</th>
          <th scope="col">Name</th>
          <th scope="col">Score</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in items"
          :key="row.rank"
          :class="{ current: row.rank === highlight }"
          :aria-current="row.rank === highlight ? 'true' : undefined"
        >
          <td>{{ row.rank }}</td>
          <td>{{ row.name }}</td>
          <td>{{ row.score.toLocaleString('en-US') }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
