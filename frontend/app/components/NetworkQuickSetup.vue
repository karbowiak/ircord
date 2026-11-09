<script setup lang="ts">
/** Network dialog, first connection: just a nickname and display name, with a summary of the preset. */
const { form, providedPassword, advanced, setupNick, nickMax, nickProblem } = useNetworkForm()
</script>
<template>
  <p class="connection-setup-intro mt-0 mr-0 mb-4.5 ml-0 text-body leading-[1.65]">
    {{ form.name ? $t('networkForm.readyIntro', { name: form.name }) : $t('networkForm.intro') }}
  </p>
  <AppField
    id="setup-nick"
    :label="$t('networkForm.nickname')"
    class="connection-setup-nick"
    :help="form.nick && nickProblem ? nickProblem : $t('networkForm.nicknameHelp')"
    :help-class="{ 'text-destructive': form.nick && nickProblem }"
  >
    <template #default="control">
      <UiInput
        v-bind="control"
        :ref="(el) => (setupNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
        v-model="form.nick"
        :placeholder="$t('mediaUi.networkSettings.quickNickPlaceholder')"
        required
        :maxlength="nickMax"
        autocomplete="nickname"
        autocapitalize="none"
        :aria-invalid="!!form.nick && !!nickProblem"
        spellcheck="false"
      />
    </template>
  </AppField>
  <AppInput
    id="setup-realname"
    v-model="form.realname"
    :label="$t('networkForm.displayName')"
    :help="$t('networkForm.displayNameHelp')"
    :placeholder="$t('networkForm.displayNamePlaceholder')"
    maxlength="100"
    autocomplete="off"
  />
  <p
    v-if="form.host"
    class="field-hint connection-setup-summary text-[12px] text-faint block leading-[1.5] mt-4 mr-0 mb-2 ml-0 wrap-anywhere"
  >
    {{ form.host }}:{{ form.port }} · {{ form.tls ? 'TLS' : $t('networkForm.plaintext')
    }}<template v-if="form.autoJoin"> · {{ form.autoJoin.split('\n').join(', ') }}</template>
    <template v-if="providedPassword"> · {{ $t('networkForm.passwordProvided') }}</template>
  </p>
  <UiButton
    variant="link"
    size="text"
    type="button"
    class="connection-setup-toggle inline-flex items-center gap-1.5 justify-self-start"
    :aria-expanded="advanced"
    aria-controls="network-connection-settings"
    @click="advanced = !advanced"
  >
    <AppIcon
      :name="advanced ? 'down' : 'right'"
      :size="14"
    />{{ $t('networkForm.connectionSettings') }}
  </UiButton>
</template>
