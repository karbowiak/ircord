<script setup vapor lang="ts">
import type { NetworkInput } from '../../../shared/types/chat'
/**
 * Adding or editing a network, or the first connection's short setup. The form's state lives in
 * useNetworkForm; each fieldset is its own component sharing it.
 */
const props = withDefaults(
  defineProps<{ networkId?: string; prefill?: Partial<NetworkInput>; firstConnection?: boolean }>(),
  { networkId: '', prefill: () => ({}), firstConnection: false },
)
const emit = defineEmits<{ close: [] }>()
const {
  dirty,
  existing,
  editing,
  presets,
  advanced,
  revealInvalidField,
  error,
  saving,
  verification,
  checkAttempt,
  needsVerification,
  selectPreset,
  submit,
  remove,
} = createNetworkForm(props, () => emit('close'))
const { t: $t } = useI18n()
// Escape or an outside click would otherwise throw away a typed server, identity and login.
const beforeClose = useDiscardGuard(
  () => dirty.value && !saving.value,
  () => emit('close'),
)
</script>
<template>
  <AppDialog
    :title="
      editing
        ? $t('networkForm.editTitle', { name: existing?.name ?? '' })
        : firstConnection
          ? $t('networkForm.firstTitle')
          : $t('networks.add')
    "
    :wide="!firstConnection || advanced"
    :before-close="beforeClose"
    @close="emit('close')"
  >
    <form
      class="dialog-form network-form pt-0 pr-5 pl-5 [&_.field-label]:mt-2.5 [&_fieldset]:mt-3 [&_fieldset]:rounded-[var(--radius-large)] [&_fieldset]:border-0 [&_fieldset]:bg-card [&_fieldset]:px-4 [&_fieldset]:pt-3 [&_fieldset]:pb-4 [&_fieldset]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] [&_legend]:float-left [&_legend]:w-full [&_legend]:p-0 [&_legend]:text-[12px] [&_legend]:font-[650] [&_legend]:uppercase [&_legend]:tracking-[0.05em] [&_legend]:text-faint [&_legend+*]:clear-both"
      @submit.prevent="submit"
      @invalid.capture="revealInvalidField"
    >
      <NetworkQuickSetup v-if="firstConnection" />
      <div
        v-show="!firstConnection || advanced"
        id="network-connection-settings"
        class="network-connection-settings flow-root"
      >
        <div
          v-if="!editing && presets.length"
          class="network-presets flex gap-1.5 flex-wrap"
        >
          <button
            v-for="preset in presets"
            :key="preset.name"
            type="button"
            class="flex items-center gap-2 rounded-full bg-secondary py-1 pr-2.5 pl-1 text-[13px] hover:bg-accent"
            @click="selectPreset(preset)"
          >
            <span
              aria-hidden="true"
              class="flex size-6 items-center justify-center rounded-full text-[12px] font-[650]"
              :class="`tone-${preset.color}`"
              >{{ preset.initial }}</span
            >{{ preset.name }}
          </button>
        </div>
        <NetworkServerFields />
        <NetworkIdentityFields />
        <NetworkLoginFields />
        <NetworkOnConnectFields />
      </div>

      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <FieldHint
        v-if="needsVerification && (verification.detail || verification.state === 'checking')"
        :class="{ 'text-destructive': verification.state === 'error' }"
        role="status"
      >
        {{
          verification.state === 'checking'
            ? $t('networkForm.verifyingAddress')
            : verification.detail
        }}
        <UiButton
          v-if="verification.state === 'error'"
          variant="link"
          size="text"
          type="button"
          @click="checkAttempt++"
        >
          {{ $t('networkForm.retryVerification') }}
        </UiButton>
      </FieldHint>
      <!-- Stays in view while the form scrolls, so saving never needs a trip to the bottom. -->
      <div
        class="form-actions sticky bottom-0 z-[1] -mx-5 mt-4 flex items-center justify-end gap-2 border-t [border-top-style:solid] border-t-border/60 bg-background px-5 py-3.5"
      >
        <UiButton
          v-if="editing"
          variant="secondary-destructive"
          type="button"
          @click="remove"
        >
          {{ $t('networkForm.removeNetwork') }}
        </UiButton>
        <span class="flex-1" />
        <UiButton
          variant="secondary"
          type="button"
          @click="emit('close')"
        >
          {{ firstConnection ? $t('networkForm.setupLater') : $t('common.cancel') }}</UiButton
        ><UiButton :disabled="saving || (needsVerification && verification.state !== 'verified')">
          {{
            verification.state === 'checking'
              ? $t('networkForm.verifying')
              : saving
                ? $t('common.saving')
                : editing
                  ? $t('common.saveChanges')
                  : firstConnection
                    ? $t('networkForm.connect')
                    : $t('networkForm.addNetwork')
          }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
