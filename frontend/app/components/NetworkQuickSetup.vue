<script setup lang="ts">
/** Network dialog, first connection: just a nickname and display name, with a summary of the preset. */
const { form, providedPassword, advanced, setupNick, nickMax, nickProblem } = useNetworkForm()
</script>
<template>
  <p class="connection-setup-intro mt-0 mr-0 mb-4.5 ml-0 text-body leading-[1.65]">
    {{ form.name ? `${form.name} is ready to connect.` : 'Let’s connect you to IRC.' }}
    Choose a nickname and an optional display name.
  </p>
  <div class="connection-setup-nick">
    <FieldLabel for="setup-nick">Nickname</FieldLabel>
    <UiInput
      id="setup-nick"
      :ref="(el) => (setupNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
      v-model="form.nick"
      placeholder="your-nick"
      required
      :maxlength="nickMax"
      autocomplete="nickname"
      autocapitalize="none"
      :aria-invalid="!!form.nick && !!nickProblem"
      aria-describedby="network-nick-hint"
      spellcheck="false"
    />
    <FieldHint
      id="network-nick-hint"
      :class="{ 'danger-text': form.nick && nickProblem }"
    >
      {{ form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
    </FieldHint>
  </div>
  <div class="connection-setup-nick">
    <FieldLabel for="setup-realname">Display name</FieldLabel>
    <UiInput
      id="setup-realname"
      v-model="form.realname"
      placeholder="How people know you"
      maxlength="100"
      autocomplete="off"
      aria-describedby="setup-realname-hint"
    />
    <FieldHint id="setup-realname-hint">
      Optional. Shared as your IRC real name. Leave blank to use your nickname.
    </FieldHint>
  </div>
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
