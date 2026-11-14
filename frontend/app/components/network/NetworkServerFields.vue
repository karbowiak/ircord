<script setup vapor lang="ts">
/** Network dialog → Server: name, icon, address, port and TLS. */
const { form, existing, hostInput, hostProblem, icon, iconPreview, iconInput, chooseIcon } =
  useNetworkForm()
const { t: $t } = useI18n()
</script>
<template>
  <fieldset>
    <legend>{{ $t('networkForm.server') }}</legend>
    <div class="network-identity flex items-end gap-3.5">
      <button
        type="button"
        class="network-icon-picker relative shrink-0 w-14 h-14 mt-2.5 rounded-lg grid [place-items:center] text-[18px] font-[650] overflow-visible"
        :class="iconPreview ? '' : `tone-${existing?.color ?? 'lavender'}`"
        :aria-label="iconPreview ? $t('networkForm.changeIcon') : $t('networkForm.addIcon')"
        :title="$t('networkForm.icon')"
        @click="iconInput?.click()"
      >
        <NuxtImg
          v-if="iconPreview"
          class="size-full rounded-[inherit] object-cover"
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
        :aria-label="$t('networkForm.iconImage')"
        @change="chooseIcon"
      />
      <AppField
        id="network-name"
        :label="$t('networkForm.name')"
        class="network-name-field flex-1"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model="form.name"
            :placeholder="$t('networkForm.namePlaceholder')"
            required
            maxlength="64"
          />
          <UiButton
            v-if="iconPreview"
            variant="link"
            size="text"
            type="button"
            class="justify-self-start"
            @click="icon = null"
          >
            {{ $t('networkForm.removeIcon') }}
          </UiButton>
        </template>
      </AppField>
    </div>
    <div
      class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start] max-[700px]:[grid-template-columns:1fr]"
    >
      <AppField
        id="network-host"
        :label="$t('networkForm.hostname')"
        :error="hostProblem || undefined"
        error-live="polite"
        class="col-span-2 max-[700px]:col-auto"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            :ref="(el) => (hostInput = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
            v-model="form.host"
            placeholder="irc.example.org"
            required
          />
        </template>
      </AppField>
      <AppField
        id="network-port"
        :label="$t('networkForm.port')"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model.number="form.port"
            type="number"
            min="1"
            max="65535"
            required
          />
        </template>
      </AppField>
      <label
        class="tls-field flex items-center gap-1.5 h-9.5 self-end justify-self-start text-[13px] text-body"
        ><UiCheckbox v-model="form.tls" /><AppIcon
          name="lock"
          :size="15"
        />TLS</label
      >
    </div>
  </fieldset>
</template>
