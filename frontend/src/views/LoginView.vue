<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import LocaleSelect from '../components/LocaleSelect.vue'
import { t, type MessageKey } from '../lib/i18n'
import { useAuthStore } from '../stores/app'

const auth = useAuthStore()
const router = useRouter()
const password = ref('')
/** Message key, translated on render so it follows a language switch. */
const error = ref<MessageKey | null>(null)
const busy = ref(false)

async function submit(): Promise<void> {
  error.value = null
  busy.value = true
  try {
    await auth.login(password.value)
    await router.replace('/admin')
  } catch (err) {
    if (err instanceof ApiError && err.status === 429) {
      error.value = 'common.tooManyAttempts'
    } else {
      error.value = 'login.invalid'
    }
  } finally {
    busy.value = false
    password.value = ''
  }
}
</script>

<template>
  <main class="flex h-full items-start justify-center p-4 pt-[var(--modal-top)]">
    <form class="card flex w-full max-w-sm flex-col" data-testid="login-form" @submit.prevent="submit">
      <h1 class="panel-title"><i class="mdi mdi-shield-lock"></i> Admin</h1>
      <div class="flex flex-col gap-4 p-6">
        <label class="flex flex-col gap-1 text-sm text-text">
          {{ t('common.password') }}
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            class="field"
            data-testid="login-password"
          />
        </label>
        <p v-if="error" class="m-0 text-sm text-danger" role="alert">{{ t(error) }}</p>
        <div class="flex items-center justify-between gap-2">
          <RouterLink to="/" class="text-sm text-link underline">{{
            t('common.backToDashboard')
          }}</RouterLink>
          <button type="submit" :disabled="busy" class="btn btn-primary">
            <i class="mdi mdi-login"></i> {{ t('login.submit') }}
          </button>
        </div>
        <LocaleSelect class="self-end" />
      </div>
    </form>
  </main>
</template>
