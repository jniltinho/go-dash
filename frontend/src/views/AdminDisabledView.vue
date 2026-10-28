<script setup lang="ts">
import LocaleSelect from '../components/LocaleSelect.vue'
import { t } from '../lib/i18n'
</script>

<template>
  <main class="flex h-full items-center justify-center p-4" data-testid="admin-disabled">
    <section class="card flex max-w-lg flex-col text-text">
      <h1 class="panel-title"><i class="mdi mdi-lock-off"></i> {{ t('adminDisabled.title') }}</h1>
      <div class="flex flex-col gap-3 p-6">
        <p class="m-0">{{ t('adminDisabled.intro') }}</p>
        <ul class="m-0 list-disc pl-5 text-sm">
          <li><code>go-dash admin set-password</code></li>
          <li>
            {{ t('adminDisabled.envVar') }} <code>DASHBOARD_ADMIN_PASSWORD_HASH</code>
            {{ t('adminDisabled.bcryptHash') }}
          </li>
          <li>{{ t('adminDisabled.envVar') }} <code>DASHBOARD_ADMIN_PASSWORD</code></li>
        </ul>
        <p class="m-0 text-sm text-muted">{{ t('adminDisabled.cli') }} <code>go-dash menu --help</code>.</p>
        <div class="flex items-center justify-between gap-2">
          <RouterLink to="/" class="text-sm text-link underline">{{
            t('common.backToDashboard')
          }}</RouterLink>
          <LocaleSelect />
        </div>
      </div>
    </section>
  </main>
</template>
