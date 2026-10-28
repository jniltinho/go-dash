<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

import { t } from '../lib/i18n'

/**
 * Modal dialog shell shared by every Vue modal: backdrop, title bar with a
 * close button, focus trap, Escape to close, the app behind made inert and
 * focus restored on close. Placement (10% from the top) lives in modal.css.
 */
const props = withDefaults(
  defineProps<{
    /** Title text (rendered as text). */
    title: string
    /** MDI icon class for the title bar, e.g. "mdi mdi-cog". */
    icon?: string
    /** Width at UI scale 1, in pixels. */
    width?: number
    testid?: string
  }>(),
  { icon: undefined, width: 880, testid: undefined },
)

const emit = defineEmits<{ close: [] }>()

const FOCUSABLE = 'button, input, select, textarea, a[href], [tabindex]'
let lastId = 0
const titleId = `modal-title-${++lastId}-${Math.random().toString(36).slice(2, 8)}`

const dialog = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
let appRoot: HTMLElement | null = null

function focusable(): HTMLElement[] {
  if (!dialog.value) return []
  return [...dialog.value.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(
    (el) => el.tabIndex >= 0 && !el.hasAttribute('disabled') && el.offsetParent !== null,
  )
}

/** Keeps Tab inside the dialog and closes it on Escape (unless a DHTMLX confirm is on top). */
function onDocumentKeydown(event: KeyboardEvent): void {
  if (document.querySelector('.dhx_alert')) return
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
    return
  }
  if (event.key !== 'Tab' || !dialog.value) return
  const elements = focusable()
  const first = elements[0]
  const last = elements[elements.length - 1]
  if (!first || !last) return
  const inside = dialog.value.contains(document.activeElement)
  if (event.shiftKey && (document.activeElement === first || !inside)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (document.activeElement === last || !inside)) {
    event.preventDefault()
    first.focus()
  }
}

onMounted(async () => {
  previousFocus = document.activeElement as HTMLElement | null
  // The dialog is teleported to <body>; make the app behind it inert.
  appRoot = document.getElementById('app')
  appRoot?.setAttribute('inert', '')
  document.addEventListener('keydown', onDocumentKeydown)
  await nextTick()
  // Content may focus its own field on mount; otherwise focus the dialog.
  if (!dialog.value?.contains(document.activeElement)) dialog.value?.focus()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onDocumentKeydown)
  appRoot?.removeAttribute('inert')
  previousFocus?.focus()
})

defineExpose({ dialog })
</script>

<template>
  <Teleport to="body">
    <div class="modal-backdrop" :data-testid="props.testid" @click.self="emit('close')">
      <div
        ref="dialog"
        class="card modal-panel"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        :style="{ '--modal-width': `${props.width}px` }"
      >
        <h2 :id="titleId" class="panel-title">
          <i v-if="props.icon" :class="props.icon" aria-hidden="true"></i> {{ props.title }}
          <span class="flex-1"></span>
          <button type="button" class="modal-close" :aria-label="t('common.close')" @click="emit('close')">
            <i class="mdi mdi-close" aria-hidden="true"></i>
          </button>
        </h2>
        <slot />
      </div>
    </div>
  </Teleport>
</template>
