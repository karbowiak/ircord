<script setup vapor lang="ts">
/** Network dialog → Server: name, icon, address, port and TLS. */
const { form, existing, hostInput, hostProblem, icon, iconPreview, iconInput, chooseIcon } =
  useNetworkForm()
const { t: $t } = useI18n()
</script>
<template>
  <fieldset>
    <legend>{{ $t('networkForm.server') }}</legend>
    <!-- The icon beside the name, then the address on one row, then the connection's security. -->
    <div class="network-identity flex items-center gap-3.5 pt-3.5">
      <!-- Remove sits on the icon itself, so the name field never changes height. -->
      <div class="relative shrink-0">
        <button
          type="button"
          class="network-icon-picker relative grid size-16 shrink-0 [place-items:center] overflow-visible rounded-xl text-[20px] font-[650]"
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
            class="icon-picker-badge absolute right-[-4px] bottom-[-4px] h-5 w-5 rounded-full bg-primary p-[3px] text-primary-foreground"
          />
        </button>
        <button
          v-if="iconPreview"
          type="button"
          class="absolute -top-1.5 -left-1.5 grid size-5 place-items-center rounded-full bg-secondary text-muted-foreground shadow-control hover:bg-destructive hover:text-white"
          :aria-label="$t('networkForm.removeIcon')"
          :title="$t('networkForm.removeIcon')"
          @click="icon = null"
        >
          <AppIcon
            name="close"
            :size="12"
          />
        </button>
      </div>
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
        class="network-name-field flex-1 [&_.field-label]:mt-0!"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model="form.name"
            :placeholder="$t('networkForm.namePlaceholder')"
            required
            maxlength="64"
          />
        </template>
      </AppField>
    </div>
    <div
      class="field-grid mt-1 grid [grid-template-columns:minmax(0,1fr)_calc(7.5rem*var(--interface-scale,1))] gap-x-3 [align-items:start]"
    >
      <AppField
        id="network-host"
        :label="$t('networkForm.hostname')"
        :error="hostProblem || undefined"
        error-live="polite"
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
    </div>
    <label
      class="tls-field mt-4 flex items-center gap-3 border-t [border-top-style:solid] border-t-border/60 pt-3.5"
    >
      <span
        class="grid size-8 shrink-0 place-items-center rounded-full"
        :class="form.tls ? 'bg-success/15 text-success' : 'bg-secondary text-faint'"
        ><AppIcon
          name="lock"
          :size="15"
      /></span>
      <span class="flex min-w-0 flex-1 flex-col gap-[1px]"
        ><strong class="text-[14px] font-[600]">{{ $t('networkForm.tls') }}</strong
        ><small class="text-[12px] text-muted-foreground">{{
          $t('networkForm.tlsHelp')
        }}</small></span
      >
      <AppSwitch
        v-model="form.tls"
        :aria-label="$t('networkForm.tls')"
      />
    </label>
  </fieldset>
</template>
