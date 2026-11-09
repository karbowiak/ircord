<script setup lang="ts">
import type { ScoreEntry } from '../../shared/utils/hash-field-scores'

/** The Hash Field top 10, with this game's entry highlighted when it placed. */
defineProps<{
  items: ScoreEntry[] | null | undefined
  loading: boolean
  /** Rank to highlight; 0 for none. */
  highlight: number
}>()
const { t, n } = useI18n()
</script>
<template>
  <section
    class="hash-field-table grid gap-1.5 w-[min(100%,_360px)]"
    aria-labelledby="hash-field-table-title"
  >
    <h5 id="hash-field-table-title">{{ t('collectionsUi.hashField.highScores') }}</h5>
    <SettingsDescription v-if="items === null">
      {{ t('collectionsUi.hashField.scoresUnavailable') }}
    </SettingsDescription>
    <SettingsDescription v-else-if="!items">{{
      t('collectionsUi.hashField.loading')
    }}</SettingsDescription>
    <table
      v-else
      :aria-busy="loading"
    >
      <thead>
        <tr>
          <th
            scope="col"
            :aria-label="t('collectionsUi.hashField.rank')"
          >
            #
          </th>
          <th scope="col">{{ t('collectionsUi.hashField.name') }}</th>
          <th scope="col">{{ t('collectionsUi.hashField.score') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in items"
          :key="row.rank"
          :class="{ current: row.rank === highlight }"
          :aria-current="row.rank === highlight ? 'true' : undefined"
        >
          <td>{{ n(row.rank) }}</td>
          <td>{{ row.name }}</td>
          <td>{{ n(row.score) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
