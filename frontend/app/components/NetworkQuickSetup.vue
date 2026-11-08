<script setup lang="ts">
/** Network dialog, first connection: just a nickname and display name, with a summary of the preset. */
const { form, providedPassword, advanced, setupNick, nickMax, nickProblem } = useNetworkForm()
</script>
<template>
  <p class="connection-setup-intro mt-0 mr-0 mb-4.5 ml-0 text-body leading-[1.65]">
    {{ form.name ? `${form.name} is ready to connect.` : 'Let’s connect you to IRC.' }}
    Choose a nickname and an optional display name.
  </p>
  <AppField
    id="setup-nick"
    label="Nickname"
    class="connection-setup-nick"
    :help="form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.'"
    :help-class="{ 'text-destructive': form.nick && nickProblem }"
  >
    <template #default="control">
      <UiInput
        v-bind="control"
        :ref="(el) => (setupNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
        v-model="form.nick"
        placeholder="your-nick"
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
    label="Display name"
    help="Optional. Shared as your IRC real name. Leave blank to use your nickname."
    placeholder="How people know you"
    maxlength="100"
    autocomplete="off"
  />
  <p
    v-if="form.host"
    class="field-hint connection-setup-summary text-[12px] text-faint block leading-[1.5] mt-4 mr-0 mb-2 ml-0 wrap-anywhere"
  >
    {{ form.host }}:{{ form.port }} · {{ form.tls ? 'TLS' : 'Plaintext'
    }}<template v-if="form.autoJoin"> · {{ form.autoJoin.split('\n').join(', ') }}</template>
    <template v-if="providedPassword"> · Server password provided</template>
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
    />Connection settings
  </UiButton>
</template>
