<script setup lang="ts">
/** Network dialog → On connect: channels to join and commands to send. */
const { form } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>{{ $t('networkForm.onConnect') }}</legend>
    <div
      class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start] max-[700px]:[grid-template-columns:1fr]"
    >
      <AppInput
        id="network-autojoin"
        v-model="form.autoJoin"
        :label="$t('networkForm.autoJoin')"
        multiline
        rows="4"
        placeholder="#channel&#10;#private secretkey"
        spellcheck="false"
      />
      <AppInput
        id="network-perform"
        v-model="form.perform"
        :label="$t('networkForm.perform')"
        multiline
        class="font-code text-[12px]"
        rows="4"
        placeholder="MODE mynick +x"
        spellcheck="false"
      />
    </div>
    <FieldHint> {{ $t('networkForm.commandsHelp') }} </FieldHint>
  </fieldset>
</template>
