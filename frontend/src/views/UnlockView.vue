<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'

import { ApiError } from '../api/client'
import LocaleSelect from '../components/LocaleSelect.vue'
import { t, type MessageKey } from '../lib/i18n'
import { useDashboardStore } from '../stores/app'

defineProps<{
  /** Title of the protected dashboard. */
  title: string
}>()

const dashboard = useDashboardStore()
const password = ref('')
/** Message key, translated on render so it follows a language switch. */
const error = ref<MessageKey | null>(null)
const busy = ref(false)
const input = ref<HTMLInputElement | null>(null)

onMounted(() => input.value?.focus())

async function submit(): Promise<void> {
  error.value = null
  busy.value = true
  try {
    await dashboard.unlock(password.value)
  } catch (err) {
    if (err instanceof ApiError && err.status === 429) {
      error.value = 'common.tooManyAttempts'
    } else if (err instanceof ApiError && err.status === 401) {
      error.value = 'unlock.wrongPassword'
    } else {
      error.value = 'unlock.failed'
    }
    await nextTick()
    input.value?.focus()
  } finally {
    busy.value = false
    password.value = ''
  }
}
</script>

<template>
  <main class="flex h-full items-start justify-center p-4 pt-[var(--modal-top)]">
    <form class="card flex w-full max-w-sm flex-col" data-testid="unlock-form" @submit.prevent="submit">
      <h1 class="panel-title">
        <i class="mdi mdi-lock" aria-hidden="true"></i>
        <span data-testid="unlock-title">{{ title }}</span>
      </h1>
      <div class="flex flex-col gap-4 p-6">
        <p class="m-0 text-sm text-text">{{ t('unlock.protected') }}</p>
        <label class="flex flex-col gap-1 text-sm text-text">
          {{ t('common.password') }}
          <input
            ref="input"
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            class="field"
            data-testid="unlock-password"
          />
        </label>
        <p v-if="error" class="m-0 text-sm text-danger" role="alert" data-testid="unlock-error">
          {{ t(error) }}
        </p>
        <div class="flex items-center justify-between gap-2">
          <RouterLink to="/admin" class="text-sm text-link underline">Admin</RouterLink>
          <button type="submit" :disabled="busy" class="btn btn-primary" data-testid="unlock-submit">
            <i class="mdi mdi-lock-open-variant" aria-hidden="true"></i> {{ t('unlock.submit') }}
          </button>
        </div>
        <LocaleSelect class="self-end" />
      </div>
    </form>
  </main>
</template>
