<script setup lang="ts">
/** Files waiting to be sent with the next message, with upload progress. */
const uploads = useUploads()
</script>
<template>
  <ul
    v-if="uploads.pending.value.length"
    class="upload-tray"
    aria-label="Files to send"
  >
    <li
      v-for="item in uploads.pending.value"
      :key="item.key"
      class="upload-item"
      :class="`is-${item.status}`"
    >
      <img
        v-if="item.preview"
        :src="item.preview"
        alt=""
      />
      <span
        v-else
        class="upload-icon"
        ><AppIcon
          name="file"
          :size="26"
      /></span>
      <span class="upload-name">{{ item.file.name }}</span>
      <span class="upload-meta">{{
        item.status === 'failed'
          ? item.error
          : item.status === 'uploading'
            ? `${Math.round(item.progress * 100)}%`
            : formatBytes(item.file.size)
      }}</span>
      <span
        v-if="item.status === 'uploading'"
        class="upload-progress"
        :style="{ width: `${item.progress * 100}%` }"
      />
      <button
        class="icon-button upload-remove"
        :aria-label="`Remove ${item.file.name}`"
        :disabled="item.status === 'uploading'"
        @click="uploads.remove(item.key)"
      >
        <AppIcon
          name="trash"
          :size="15"
        />
      </button>
    </li>
  </ul>
</template>
