<script setup lang="ts">
/** Network dialog → Server: name, icon, address, port and TLS. */
const { form, existing, icon, iconPreview, iconInput, chooseIcon } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>Server</legend>
    <div class="network-identity flex items-end gap-3.5">
      <button
        type="button"
        class="network-icon-picker relative shrink-0 w-14 h-14 mt-2.5 rounded-lg grid [place-items:center] text-[18px] font-[650] overflow-visible"
        :class="iconPreview ? '' : `tone-${existing?.color ?? 'lavender'}`"
        :aria-label="iconPreview ? 'Change network icon' : 'Add a network icon'"
        title="Network icon"
        @click="iconInput?.click()"
      >
        <img
          v-if="iconPreview"
          :src="iconPreview"
          alt=""
        /><span v-else>{{ (form.name || '?').slice(0, 2).toUpperCase() }}</span>
        <AppIcon
          name="image"
          :size="14"
          class="icon-picker-badge absolute right-[-4px] bottom-[-4px] p-[3px] w-5 h-5 rounded-full bg-primary text-primary-foreground"
        />
      </button>
      <input
        ref="iconInput"
        type="file"
        class="sr-only"
        accept="image/png,image/jpeg,image/webp,image/gif"
        aria-label="Network icon image"
        @change="chooseIcon"
      />
      <div class="network-name-field flex-1">
        <FieldLabel for="network-name">Name</FieldLabel
        ><UiInput
          id="network-name"
          v-model="form.name"
          placeholder="My community"
          required
          maxlength="40"
        />
        <UiButton
          variant="link"
          size="text"
          v-if="iconPreview"
          type="button"
          @click="icon = null"
        >
          Remove icon
        </UiButton>
      </div>
    </div>
    <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
      <div class="span-2">
        <FieldLabel for="network-host">Hostname</FieldLabel
        ><UiInput
          id="network-host"
          v-model="form.host"
          placeholder="irc.example.org"
          required
          pattern="[a-zA-Z0-9.\-]+"
        />
      </div>
      <div>
        <FieldLabel for="network-port">Port</FieldLabel
        ><UiInput
          id="network-port"
          v-model.number="form.port"
          type="number"
          min="1"
          max="65535"
          required
        />
      </div>
      <label class="tls-field flex items-center gap-1.5 h-9.5 text-[13px] text-body"
        ><UiCheckbox v-model="form.tls" /><AppIcon
          name="lock"
          :size="15"
        />TLS</label
      >
    </div>
  </fieldset>
</template>
