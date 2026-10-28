<script setup lang="ts">
import * as dhx from '@dhtmlx/suite'
import { computed, nextTick, onMounted, ref } from 'vue'

import { api, ApiError } from '../api/client'
import type { Dashboard } from '../api/types'
import { dashboardPath, normalizeSlug, RESERVED_SLUGS, SLUG_PATTERN } from '../lib/dashboard'
import { t } from '../lib/i18n'
import { translateError } from '../lib/messages'
import AppModal from './AppModal.vue'

const props = defineProps<{
  dashboards: Dashboard[]
  /** Configured title, used by the default dashboard while its own title is empty. */
  configTitle: string
}>()

const emit = defineEmits<{
  /** Something was saved: the parent reloads the list. */
  changed: []
  close: []
}>()

const titleInput = ref<HTMLInputElement | null>(null)
const passwordInput = ref<HTMLInputElement | null>(null)

/** Dashboard being edited; null = creating a new one. */
const editing = ref<Dashboard | null>(null)
const slug = ref('')
const title = ref('')
/** Dashboard whose password is being set. */
const passwordFor = ref<Dashboard | null>(null)
const password = ref('')
const passwordRepeat = ref('')
const error = ref<{ field: string | null; text: string } | null>(null)
const busy = ref(false)

const isDefault = computed(() => editing.value?.is_default ?? false)
const slugHint = computed(() => {
  const value = normalizeSlug(slug.value)
  if (isDefault.value || value === '') return ''
  if (!SLUG_PATTERN.test(value)) return t('dashboards.slugInvalid')
  if (RESERVED_SLUGS.has(value)) return t('dashboards.slugReserved', { slug: value })
  return t('dashboards.slugAddress', { path: dashboardPath(value) })
})

function displayTitle(d: Dashboard): string {
  return d.title || t('dashboards.configTitle', { title: props.configTitle })
}

function notify(text: string): void {
  dhx.message({ text, expire: 4000 })
}

function fail(err: unknown): void {
  if (err instanceof ApiError) {
    error.value = { field: err.field ?? null, text: translateError(err.message) }
    return
  }
  error.value = { field: null, text: t('common.unexpectedError') }
}

async function startEdit(d: Dashboard | null): Promise<void> {
  error.value = null
  passwordFor.value = null
  editing.value = d
  slug.value = d?.slug ?? ''
  title.value = d?.title ?? ''
  await nextTick()
  titleInput.value?.focus()
}

async function save(): Promise<void> {
  error.value = null
  busy.value = true
  const input = { slug: isDefault.value ? null : normalizeSlug(slug.value), title: title.value.trim() }
  try {
    if (editing.value) {
      await api.updateDashboard(editing.value.id, input)
      notify(t('dashboards.saved'))
    } else {
      await api.createDashboard(input)
      notify(t('dashboards.created'))
    }
    emit('changed')
    await startEdit(null)
  } catch (err) {
    fail(err)
  } finally {
    busy.value = false
  }
}

async function remove(d: Dashboard): Promise<void> {
  const items = d.item_count
  const ok = await dhx.confirm({
    // Titles are user data: render them as text, never as HTML.
    htmlEnable: false,
    header: t('dashboards.deleteHeader'),
    text: items
      ? t('dashboards.deleteWithItems', { title: d.title, count: items, path: dashboardPath(d.slug) })
      : t('common.deleteConfirm', { title: d.title }),
    buttons: [t('common.cancel'), t('common.delete')],
  })
  if (!ok) return
  try {
    await api.deleteDashboard(d.id, items > 0)
    notify(t('dashboards.deleted'))
    if (editing.value?.id === d.id) await startEdit(null)
    emit('changed')
  } catch (err) {
    fail(err)
  }
}

async function askPassword(d: Dashboard): Promise<void> {
  error.value = null
  passwordFor.value = d
  password.value = ''
  passwordRepeat.value = ''
  await nextTick()
  passwordInput.value?.focus()
}

async function savePassword(): Promise<void> {
  const d = passwordFor.value
  if (!d) return
  error.value = null
  if (password.value !== passwordRepeat.value) {
    error.value = { field: 'password', text: t('dashboards.passwordMismatch') }
    return
  }
  busy.value = true
  try {
    await api.setDashboardPassword(d.id, password.value)
    notify(t('dashboards.passwordSet', { title: d.title || props.configTitle }))
    passwordFor.value = null
    emit('changed')
  } catch (err) {
    fail(err)
  } finally {
    busy.value = false
    password.value = ''
    passwordRepeat.value = ''
  }
}

async function clearPassword(d: Dashboard): Promise<void> {
  const ok = await dhx.confirm({
    // Titles are user data: render them as text, never as HTML.
    htmlEnable: false,
    header: t('dashboards.removePassword'),
    text: t('dashboards.removePasswordText', {
      title: d.title || props.configTitle,
      path: dashboardPath(d.slug),
    }),
    buttons: [t('common.cancel'), t('dashboards.removePassword')],
  })
  if (!ok) return
  try {
    await api.clearDashboardPassword(d.id)
    notify(t('dashboards.passwordRemoved'))
    emit('changed')
  } catch (err) {
    fail(err)
  }
}

onMounted(() => startEdit(null))
</script>

<template>
  <AppModal
    title="Dashboards"
    icon="mdi mdi-view-dashboard-variant"
    :width="960"
    testid="dashboards-panel"
    @close="emit('close')"
  >
    <div class="modal-body">
      <table class="dash-table">
        <thead>
          <tr>
            <th scope="col">{{ t('dashboards.colTitle') }}</th>
            <th scope="col">{{ t('dashboards.colAddress') }}</th>
            <th scope="col">{{ t('dashboards.colItems') }}</th>
            <th scope="col">{{ t('dashboards.colAccess') }}</th>
            <th scope="col">
              <span class="sr-only">{{ t('dashboards.colActions') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in dashboards" :key="d.id" :data-testid="`dash-row-${d.slug ?? 'default'}`">
            <td>{{ displayTitle(d) }}</td>
            <td>
              <code>{{ dashboardPath(d.slug) }}</code>
            </td>
            <td>{{ d.item_count }}</td>
            <td>
              <span v-if="d.protected"
                ><i class="mdi mdi-lock" aria-hidden="true"></i> {{ t('dashboards.protected') }}</span
              >
              <span v-else
                ><i class="mdi mdi-earth" aria-hidden="true"></i> {{ t('dashboards.public') }}</span
              >
            </td>
            <td>
              <div class="dash-actions">
                <button type="button" class="btn btn-secondary" data-testid="dash-edit" @click="startEdit(d)">
                  <i class="mdi mdi-pencil" aria-hidden="true"></i> {{ t('common.edit') }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary"
                  data-testid="dash-set-password"
                  @click="askPassword(d)"
                >
                  <i class="mdi mdi-key" aria-hidden="true"></i> {{ t('dashboards.setPassword') }}
                </button>
                <button
                  v-if="d.protected"
                  type="button"
                  class="btn btn-secondary"
                  data-testid="dash-clear-password"
                  @click="clearPassword(d)"
                >
                  <i class="mdi mdi-lock-open-variant" aria-hidden="true"></i>
                  {{ t('dashboards.removePassword') }}
                </button>
                <button
                  v-if="!d.is_default"
                  type="button"
                  class="btn btn-secondary"
                  data-testid="dash-delete"
                  @click="remove(d)"
                >
                  <i class="mdi mdi-delete" aria-hidden="true"></i> {{ t('common.delete') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <form
        v-if="passwordFor"
        class="dash-form"
        data-testid="dash-password-form"
        @submit.prevent="savePassword"
      >
        <h3 class="m-0 text-sm font-semibold text-text">
          {{
            t('dashboards.passwordFor', {
              title: passwordFor.title || configTitle,
              path: dashboardPath(passwordFor.slug),
            })
          }}
        </h3>
        <div class="dash-fields">
          <label class="flex flex-col gap-1 text-sm text-text">
            {{ t('dashboards.newPassword') }}
            <input
              ref="passwordInput"
              v-model="password"
              type="password"
              autocomplete="new-password"
              required
              minlength="8"
              maxlength="72"
              class="field"
              data-testid="dash-password"
            />
          </label>
          <label class="flex flex-col gap-1 text-sm text-text">
            {{ t('dashboards.repeatPassword') }}
            <input
              v-model="passwordRepeat"
              type="password"
              autocomplete="new-password"
              required
              class="field"
              data-testid="dash-password-repeat"
            />
          </label>
        </div>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="passwordFor = null">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="busy" data-testid="dash-password-save">
            <i class="mdi mdi-content-save" aria-hidden="true"></i> {{ t('dashboards.savePassword') }}
          </button>
        </div>
      </form>

      <form v-else class="dash-form" data-testid="dash-form" @submit.prevent="save">
        <h3 class="m-0 text-sm font-semibold text-text">
          <template v-if="editing">{{ t('dashboards.editing', { title: displayTitle(editing) }) }}</template>
          <template v-else>{{ t('dashboards.new') }}</template>
        </h3>
        <div class="dash-fields">
          <label class="flex flex-col gap-1 text-sm text-text">
            {{ t('dashboards.titleLabel') }}
            <input
              ref="titleInput"
              v-model="title"
              type="text"
              maxlength="120"
              :required="!isDefault"
              :placeholder="isDefault ? configTitle : t('dashboards.titlePlaceholder')"
              class="field"
              data-testid="dash-title"
            />
          </label>
          <label class="flex flex-col gap-1 text-sm text-text">
            {{ t('dashboards.slugLabel') }}
            <input
              v-model="slug"
              type="text"
              maxlength="40"
              :disabled="isDefault"
              :required="!isDefault"
              :placeholder="isDefault ? t('dashboards.slugDefault') : t('dashboards.slugPlaceholder')"
              autocapitalize="off"
              spellcheck="false"
              class="field"
              data-testid="dash-slug"
            />
          </label>
        </div>
        <p v-if="slugHint" class="m-0 text-sm text-muted" data-testid="dash-slug-hint">{{ slugHint }}</p>
        <div class="flex justify-end gap-2">
          <button v-if="editing" type="button" class="btn btn-secondary" @click="startEdit(null)">
            {{ t('dashboards.cancelEdit') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="busy" data-testid="dash-save">
            <i class="mdi mdi-content-save" aria-hidden="true"></i>
            {{ editing ? t('common.save') : t('dashboards.create') }}
          </button>
        </div>
      </form>

      <p v-if="error" class="m-0 text-sm text-danger" role="alert" data-testid="dash-error">
        {{ error.text }}
      </p>
    </div>
  </AppModal>
</template>
