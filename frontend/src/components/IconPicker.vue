<script setup lang="ts">
import { computed, nextTick, onMounted, ref, shallowRef, watch } from 'vue'

import { t } from '../lib/i18n'
import { iconClass, iconName, searchIcons, SUGGESTED_ICONS } from '../lib/icons'
import AppModal from './AppModal.vue'

const props = defineProps<{
  /** Current value, e.g. "mdi mdi-home" (or empty). */
  current: string | null
}>()

const emit = defineEmits<{
  select: [value: string | null]
  close: []
}>()

const RESULT_LIMIT = 240

const allNames = shallowRef<string[]>([])
const loading = ref(true)
const query = ref('')
const grid = ref<HTMLElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const selected = ref<string | null>(iconName(props.current))
/** Index of the grid cell that takes part in the Tab order (roving tabindex). */
const activeIndex = ref(0)

const result = computed(() => searchIcons(allNames.value, query.value, RESULT_LIMIT))
const searching = computed(() => query.value.trim() !== '')
const visible = computed(() => (searching.value ? result.value.names : SUGGESTED_ICONS))

function choose(name: string, index: number): void {
  selected.value = name
  activeIndex.value = index
}

function confirm(name = selected.value): void {
  if (name) emit('select', iconClass(name))
}

function onSearchEnter(): void {
  const first = visible.value[0]
  if (first) confirm(first)
}

/** Number of columns currently rendered by the CSS grid. */
function columns(): number {
  if (!grid.value) return 1
  return getComputedStyle(grid.value).gridTemplateColumns.split(' ').filter(Boolean).length || 1
}

function focusCell(index: number): void {
  const count = visible.value.length
  if (count === 0) return
  activeIndex.value = Math.min(Math.max(index, 0), count - 1)
  nextTick(() => grid.value?.querySelectorAll<HTMLElement>('.icon-cell')[activeIndex.value]?.focus())
}

/** Arrow keys move between cells; Enter uses the focused icon. */
function onGridKeydown(event: KeyboardEvent): void {
  const moves: Record<string, number> = {
    ArrowRight: 1,
    ArrowLeft: -1,
    ArrowDown: columns(),
    ArrowUp: -columns(),
  }
  if (event.key in moves) {
    event.preventDefault()
    focusCell(activeIndex.value + moves[event.key]!)
    return
  }
  if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault()
    focusCell(event.key === 'Home' ? 0 : visible.value.length - 1)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    confirm(visible.value[activeIndex.value])
  }
}

// A new result list starts navigation at the selected icon or the first one.
watch(
  visible,
  (names) => {
    const index = selected.value ? names.indexOf(selected.value) : -1
    activeIndex.value = Math.max(index, 0)
  },
  { immediate: true },
)

onMounted(async () => {
  await nextTick()
  searchInput.value?.focus()
  // Loaded lazily: ~7,400 names only needed while the picker is open.
  const module = await import('virtual:mdi-icons')
  allNames.value = module.default
  loading.value = false
})
</script>

<template>
  <AppModal
    :title="t('icons.title')"
    icon="mdi mdi-emoticon-outline"
    :width="880"
    testid="icon-picker"
    @close="emit('close')"
  >
    <div class="flex items-center gap-2 p-3">
      <input
        ref="searchInput"
        v-model="query"
        type="search"
        class="field flex-1"
        :placeholder="t('icons.placeholder')"
        :aria-label="t('icons.searchLabel')"
        aria-describedby="icon-picker-status"
        data-testid="icon-search"
        @keydown.enter.prevent="onSearchEnter"
        @keydown.down.prevent="focusCell(activeIndex)"
      />
    </div>

    <p
      id="icon-picker-status"
      class="m-0 px-3 pb-2 text-sm text-muted"
      aria-live="polite"
      data-testid="icon-status"
    >
      <template v-if="!searching">{{ t('icons.suggested') }}</template>
      <template v-else-if="loading">{{ t('icons.loading') }}</template>
      <template v-else-if="result.total === 0">{{ t('icons.none', { query }) }}</template>
      <template v-else-if="result.total > RESULT_LIMIT">
        {{ t('icons.truncated', { limit: RESULT_LIMIT, total: result.total }) }}
      </template>
      <template v-else>{{ t('icons.found', { total: result.total }) }}</template>
    </p>

    <div
      ref="grid"
      class="icon-grid"
      role="listbox"
      :aria-label="t('icons.gridLabel')"
      @keydown="onGridKeydown"
    >
      <button
        v-for="(name, index) in visible"
        :key="name"
        type="button"
        role="option"
        class="icon-cell"
        :class="{ 'is-selected': selected === name }"
        :aria-selected="selected === name"
        :tabindex="index === activeIndex ? 0 : -1"
        :title="name"
        :data-icon="name"
        @click="choose(name, index)"
        @dblclick="confirm(name)"
      >
        <i :class="iconClass(name)" aria-hidden="true"></i>
        <span>{{ name }}</span>
      </button>
    </div>

    <div class="context-bar icon-picker-footer">
      <span v-if="selected" class="flex items-center gap-2 text-sm" data-testid="icon-preview">
        <i :class="iconClass(selected)" class="text-2xl" aria-hidden="true"></i>
        <code>{{ iconClass(selected) }}</code>
      </span>
      <span v-else class="text-sm text-muted">{{ t('icons.noneSelected') }}</span>
      <span class="flex-1"></span>
      <button type="button" class="btn btn-secondary" @click="emit('select', null)">
        <i class="mdi mdi-close-circle-outline" aria-hidden="true"></i> {{ t('icons.noIcon') }}
      </button>
      <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-primary" :disabled="!selected" @click="confirm()">
        <i class="mdi mdi-check" aria-hidden="true"></i> {{ t('icons.use') }}
      </button>
    </div>
  </AppModal>
</template>
