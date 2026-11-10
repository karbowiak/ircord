<script setup lang="ts">
/** Files waiting to be sent with the next message, with upload progress. */
const uploads = useUploads()
const { locale } = useI18n()
</script>
<template>
  <ul
    v-if="uploads.pending.value.length"
    class="upload-tray flex gap-2 mt-0 mr-0 mb-2 ml-0 p-2.5 list-none overflow-x-auto rounded-md bg-field shadow-[inset_0_0_0_1px_var(--border)]"
    :aria-label="$t('uploads.filesToSend')"
  >
    <li
      v-for="item in uploads.pending.value"
      :key="item.key"
      class="upload-item relative shrink-0 w-37.5 p-1.5 rounded-sm bg-card grid gap-0.5 overflow-hidden"
      :class="`is-${item.status}`"
    >
      <!-- Native on purpose: a blob: object URL the upload owner revokes; nothing to resolve. -->
      <img
        v-if="item.preview"
        :src="item.preview"
        alt=""
        class="w-full h-24 object-cover rounded-[4px] bg-rail"
      />
      <span
        v-else
        class="upload-icon grid [place-items:center] text-muted-foreground w-full h-24 object-cover rounded-[4px] bg-rail"
        ><AppIcon
          name="file"
          :size="26"
      /></span>
      <span class="upload-name text-[12px] overflow-hidden text-ellipsis whitespace-nowrap">{{
        item.file.name
      }}</span>
      <span
        class="upload-meta text-[11px] overflow-hidden text-ellipsis whitespace-nowrap"
        :class="item.status === 'failed' ? 'text-destructive' : 'text-faint'"
        >{{
          item.status === 'failed'
            ? item.errorKey
              ? $t(item.errorKey)
              : item.error
            : item.status === 'uploading'
              ? $n(item.progress, { style: 'percent', maximumFractionDigits: 0 })
              : formatBytes(item.file.size, locale)
        }}</span
      >
      <span
        v-if="item.status === 'uploading'"
        class="upload-progress absolute left-0 bottom-0 h-[3px] bg-primary [transition:width_0.15s]"
        :style="{ width: `${item.progress * 100}%` }"
      />
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="upload-remove absolute top-2.5 right-2.5 size-[26px] bg-[rgb(15_15_20_/_72%)]! text-white!"
        :aria-label="$t('uploads.removeFile', { name: item.file.name })"
        :disabled="item.status === 'uploading'"
        @click="uploads.remove(item.key)"
      >
        <AppIcon
          name="trash"
          :size="15"
        />
      </UiButton>
    </li>
  </ul>
</template>
