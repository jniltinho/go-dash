<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import type { MenuNode } from '../api/types'
import { applyCardOrder, cardOrderKey, loadCardOrder, saveCardOrder } from '../lib/cardOrder'
import { t, type MessageKey } from '../lib/i18n'
import { cardTree, moveIndex } from '../lib/menu'

const props = defineProps<{
  /** Dashboard slug ("default" for "/"): each dashboard has its own card order. */
  slug: string
  /** Full public tree; only items placed on cards are shown here. */
  tree: MenuNode[]
  /** Message key of a load failure. */
  error: MessageKey | null
  loading: boolean
  highlighted: number | null
}>()

const emit = defineEmits<{
  run: [item: MenuNode]
  retry: []
  /** A card drag started/ended (the parent pauses background menu refreshes). */
  dragging: [on: boolean]
}>()

/** This browser's card order (null = default order from the admin). */
const savedOrder = ref<number[] | null>(loadCardOrder(props.slug))
const cards = computed(() => applyCardOrder(cardTree(props.tree), savedOrder.value))
/** Card being dragged by its header, and the card under the pointer. */
const dragIndex = ref<number | null>(null)
const dropIndex = ref<number | null>(null)

watch(
  () => dragIndex.value !== null,
  (on) => emit('dragging', on),
)

/** Another tab of this browser changed the order: follow it (cancelling a drag in progress). */
function onStorage(event: StorageEvent): void {
  if (event.key !== cardOrderKey(props.slug) && event.key !== null) return
  dragIndex.value = null
  dropIndex.value = null
  savedOrder.value = loadCardOrder(props.slug)
}
onMounted(() => window.addEventListener('storage', onStorage))
onBeforeUnmount(() => window.removeEventListener('storage', onStorage))

/** Collects every descendant that has a URL, depth-first. */
function links(node: MenuNode): MenuNode[] {
  const out: MenuNode[] = node.url ? [node] : []
  for (const child of node.items) out.push(...links(child))
  return out
}

/** Moves a card and saves the new order in this browser only. */
function moveCard(from: number, to: number): void {
  if (from === to || to < 0 || to >= cards.value.length) return
  savedOrder.value = moveIndex(cards.value, from, to).map((n) => n.id)
  saveCardOrder(props.slug, savedOrder.value)
}

/** Click/keyboard alternative to dragging: the header's move buttons. */
async function moveBy(index: number, delta: number): Promise<void> {
  const to = index + delta
  if (to < 0 || to >= cards.value.length) return
  const id = cards.value[index]!.id
  // Keep focus on the moved card; at an edge the pressed button is disabled, so use the other one.
  const atEdge = delta > 0 ? to === cards.value.length - 1 : to === 0
  const target = `${delta > 0 !== atEdge ? 'move-right' : 'move-left'}-${id}`
  moveCard(index, to)
  await nextTick()
  document.querySelector<HTMLButtonElement>(`[data-testid="${target}"]`)?.focus()
}

/** Dragging starts on the header (like a Grafana panel); the whole card is the drag image. */
function onDragStart(event: DragEvent, index: number): void {
  dragIndex.value = index
  const transfer = event.dataTransfer
  if (!transfer) return
  transfer.setData('text/plain', String(cards.value[index]!.id))
  transfer.effectAllowed = 'move'
  const card = (event.currentTarget as HTMLElement).closest('section')
  if (card) {
    const box = card.getBoundingClientRect()
    transfer.setDragImage(card, event.clientX - box.left, event.clientY - box.top)
  }
}

function onDragOver(event: DragEvent, index: number): void {
  if (dragIndex.value === null) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
  dropIndex.value = index
}

function onDrop(index: number): void {
  const from = dragIndex.value
  dragIndex.value = null
  dropIndex.value = null
  if (from !== null) moveCard(from, index)
}

function onDragEnd(): void {
  dragIndex.value = null
  dropIndex.value = null
}
</script>

<template>
  <div class="overflow-auto" style="padding: calc(20px * var(--ui-scale))" data-testid="home">
    <div v-if="error" class="card flex max-w-md flex-col items-start gap-3 p-4" data-testid="menu-error">
      <p class="m-0 text-danger">{{ t(error) }}</p>
      <button type="button" class="btn btn-primary" @click="emit('retry')">
        <i class="mdi mdi-refresh"></i> {{ t('common.retry') }}
      </button>
    </div>

    <!-- Only the first load replaces the grid; reloads keep it mounted (focus, scroll). -->
    <p v-else-if="loading && cards.length === 0" class="card m-0 inline-block p-3 text-muted">
      {{ t('common.loading') }}
    </p>

    <div v-else-if="cards.length === 0" class="card max-w-xl p-4 text-text">
      {{ t('home.empty') }} <RouterLink to="/admin" class="text-link underline">Admin</RouterLink>.
    </div>

    <template v-else>
      <div class="home-grid" data-testid="card-grid">
        <section
          v-for="(group, index) in cards"
          :key="group.id"
          class="card flex flex-col"
          :class="{
            'is-dragging': dragIndex === index,
            'is-drop-target': dropIndex === index && dragIndex !== null && dragIndex !== index,
          }"
          :data-testid="`card-${group.id}`"
          @dragover="onDragOver($event, index)"
          @drop.prevent="onDrop(index)"
        >
          <h2
            class="card-header card-handle m-0 text-base"
            :draggable="cards.length > 1"
            :title="cards.length > 1 ? t('home.dragHint') : undefined"
            :data-testid="`card-handle-${group.id}`"
            @dragstart="onDragStart($event, index)"
            @dragend="onDragEnd"
          >
            <i :class="group.icon ?? 'mdi mdi-folder-outline'" aria-hidden="true"></i>
            <span class="flex-1 truncate">{{ group.title }}</span>
            <!-- Shown on hover/focus (always on touch screens): grip plus click/keyboard moves. -->
            <span v-if="cards.length > 1" class="card-tools">
              <i class="mdi mdi-drag card-grip" aria-hidden="true"></i>
              <button
                type="button"
                class="card-move"
                :disabled="index === 0"
                :aria-label="t('home.moveBefore', { title: group.title })"
                :title="t('home.moveBeforeShort')"
                :data-testid="`move-left-${group.id}`"
                @click="void moveBy(index, -1)"
              >
                <i class="mdi mdi-chevron-left" aria-hidden="true"></i>
              </button>
              <button
                type="button"
                class="card-move"
                :disabled="index === cards.length - 1"
                :aria-label="t('home.moveAfter', { title: group.title })"
                :title="t('home.moveAfterShort')"
                :data-testid="`move-right-${group.id}`"
                @click="void moveBy(index, 1)"
              >
                <i class="mdi mdi-chevron-right" aria-hidden="true"></i>
              </button>
            </span>
          </h2>
          <ul class="plain card-body flex flex-1 flex-col gap-1">
            <li v-for="link in links(group)" :key="link.id">
              <button
                type="button"
                class="link-item"
                :class="{ 'is-highlighted': highlighted === link.id }"
                :title="link.description ?? link.url ?? ''"
                :data-testid="`link-${link.id}`"
                @click="emit('run', link)"
              >
                <i :class="link.icon ?? 'mdi mdi-link-variant'" aria-hidden="true"></i>
                <span class="truncate">{{ link.title }}</span>
                <i
                  v-if="link.target === 'blank'"
                  class="mdi mdi-open-in-new ml-auto text-xs"
                  :title="t('home.newTab')"
                ></i>
              </button>
            </li>
          </ul>
        </section>
      </div>
    </template>
  </div>
</template>
