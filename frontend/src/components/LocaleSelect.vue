<script setup lang="ts">
import { isLocale, LOCALES, t } from '../lib/i18n'
import { useLocaleStore } from '../stores/app'

/** Language picker for screens without the toolbar (login, unlock, errors). */
const localeStore = useLocaleStore()

function onChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  if (isLocale(value)) localeStore.set(value)
}
</script>

<template>
  <label class="flex items-center gap-2 text-sm text-text">
    <i class="mdi mdi-translate" aria-hidden="true"></i>
    <span class="sr-only">{{ t('common.language') }}</span>
    <select class="field" :value="localeStore.locale" data-testid="locale-select" @change="onChange">
      <option v-for="l in LOCALES" :key="l.locale" :value="l.locale" :lang="l.locale">{{ l.label }}</option>
    </select>
  </label>
</template>
