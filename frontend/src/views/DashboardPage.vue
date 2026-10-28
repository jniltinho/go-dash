<script setup lang="ts">
import { computed, watch } from 'vue'

import LocaleSelect from '../components/LocaleSelect.vue'
import { t } from '../lib/i18n'
import UnlockView from './UnlockView.vue'
import DashboardView from './DashboardView.vue'
import { useConfigStore, useDashboardStore, useMenuStore } from '../stores/app'

const props = defineProps<{
  /** Dashboard slug; "default" for the dashboard served at "/". */
  slug: string
}>()

const configStore = useConfigStore()
const dashboard = useDashboardStore()
const menuStore = useMenuStore()

/** Title of this dashboard, shown in the toolbar and the browser tab. */
const title = computed(() => dashboard.status?.title || configStore.config.title)

// A new dashboard starts from scratch: nothing of the previous one survives.
watch(
  () => props.slug,
  (slug) => {
    menuStore.reset(slug)
    void dashboard.open(slug)
  },
  { immediate: true },
)

// The server refused the menu while viewing (password changed, dashboard
// deleted): DashboardView unmounts, destroying its tabs, iframes and widgets.
watch(
  () => menuStore.access,
  (access) => {
    if (access !== 'ok') dashboard.accessLost(access)
  },
)

watch(
  [title, () => dashboard.state, () => t('dashboard.notFound')],
  ([value, state, notFound]) => {
    document.title = state === 'not-found' ? notFound : value
  },
  { immediate: true },
)
</script>

<template>
  <DashboardView v-if="dashboard.state === 'ready'" :key="slug" :slug="slug" :title="title" />

  <UnlockView v-else-if="dashboard.state === 'locked'" :title="title" />

  <main
    v-else-if="dashboard.state === 'not-found'"
    class="flex h-full items-center justify-center p-4"
    data-testid="dashboard-not-found"
  >
    <section class="card flex max-w-lg flex-col text-text">
      <h1 class="panel-title">
        <i class="mdi mdi-view-dashboard-outline" aria-hidden="true"></i> {{ t('dashboard.notFound') }}
      </h1>
      <div class="flex flex-col gap-3 p-6">
        <p class="m-0">
          {{ t('dashboard.notFoundBefore') }} <code>{{ `/dash/${slug}` }}</code>
          {{ t('dashboard.notFoundAfter') }}
        </p>
        <div class="flex items-center justify-between gap-2">
          <RouterLink to="/" class="text-sm text-link underline">{{ t('dashboard.goHome') }}</RouterLink>
          <LocaleSelect />
        </div>
      </div>
    </section>
  </main>

  <main v-else-if="dashboard.state === 'error'" class="flex h-full items-center justify-center p-4">
    <section class="card flex max-w-lg flex-col text-text">
      <h1 class="panel-title">
        <i class="mdi mdi-alert-outline" aria-hidden="true"></i> {{ t('dashboard.errorTitle') }}
      </h1>
      <div class="flex flex-col gap-3 p-6">
        <p class="m-0">{{ t('dashboard.loadFailed') }}</p>
        <div>
          <button type="button" class="btn btn-primary" @click="dashboard.open(slug)">
            <i class="mdi mdi-refresh" aria-hidden="true"></i> {{ t('common.retry') }}
          </button>
        </div>
      </div>
    </section>
  </main>

  <main v-else class="flex h-full items-center justify-center p-4 text-muted" aria-busy="true">
    {{ t('common.loading') }}
  </main>
</template>
