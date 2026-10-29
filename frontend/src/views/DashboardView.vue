<script setup lang="ts">
import * as dhx from '@dhtmlx/suite'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import type { MenuNode } from '../api/types'
import HomePane from '../components/HomePane.vue'
import { DETACHED, useDhx } from '../composables/useDhx'
import { currentLocale, localeMenu, parseLocaleId, t } from '../lib/i18n'
import { buildToolbar, isSameOrigin, lockAction, parseHash, parseToolbarId } from '../lib/menu'
import { onScaleChange, uiScale } from '../lib/scale'
import { TabTracker } from '../lib/tabs'
import { parseThemeId, themeMenu } from '../lib/theme'
import { useConfigStore, useDashboardStore, useLocaleStore, useMenuStore, useThemeStore } from '../stores/app'

const HOME_TAB = 'home'
/** Tab size at scale 1 (Full HD); multiplied by --ui-scale on larger screens. */
const TAB_WIDTH = 190
const TAB_HEIGHT = 26
const tabId = (id: number) => `tab-${id}`
const itemFromTab = (id: string) => (id.startsWith('tab-') ? Number(id.slice(4)) : null)

interface OpenTab {
  item: MenuNode
  /** Bumped by "Reload" to force the iframe to reload. */
  reloads: number
}

const props = defineProps<{
  /** Dashboard slug ("default" for "/"). */
  slug: string
  /** Dashboard title shown in the toolbar. */
  title: string
}>()

const router = useRouter()
const configStore = useConfigStore()
const dashboard = useDashboardStore()
const menuStore = useMenuStore()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { own } = useDhx()

const root = ref<HTMLElement | null>(null)
const panesTarget = ref<HTMLElement | null>(null)
const openTabs = ref<OpenTab[]>([])
const activeTab = ref<string>(HOME_TAB)
const highlighted = ref<number | null>(null)
/** A home card is being dragged. */
const draggingCard = ref(false)
/** How often an open dashboard checks for menu changes made by an admin. */
const MENU_SYNC_MS = 30_000
let menuSyncTimer: ReturnType<typeof setInterval> | null = null

let toolbar: dhx.Toolbar | null = null
let tabbar: dhx.Tabbar | null = null
let stopScaleWatch: (() => void) | null = null
const tracker = shallowRef(new TabTracker(10))

const sandbox = computed(() => configStore.config.iframe_sandbox)

function notify(text: string): void {
  dhx.message({ text, expire: 4000, css: 'app-message' })
}

function refreshToolbar(): void {
  if (!toolbar) return
  toolbar.data.removeAll()
  const extra = [themeMenu(themeStore.prefs), localeMenu()]
  if (dashboard.status?.protected) extra.push(lockAction())
  toolbar.data.parse(buildToolbar(props.title, menuStore.tree, extra) as never)
}

/**
 * Picks up changes made elsewhere (items, card order) without a reload. Skips
 * while the tab is hidden, a card is being dragged, or a toolbar
 * dropdown is open (rebuilding the toolbar would close it).
 */
function canSyncMenu(): boolean {
  return !document.hidden && !draggingCard.value && !document.querySelector('.dhx_widget.dhx_menu')
}

async function syncMenu(): Promise<void> {
  // Checked before the request and again when the response arrives.
  if (!canSyncMenu()) return
  if (await menuStore.refresh(canSyncMenu)) refreshToolbar()
}

function onVisibilityChange(): void {
  if (!document.hidden) void syncMenu()
}

function updateClosable(): void {
  if (!tabbar) return
  tabbar.config.closable = openTabs.value.map((t) => tabId(t.item.id))
  tabbar.paint()
}

/** Pushes a history entry so browser back/forward walks through visited tabs. */
function syncHash(tab: string): void {
  const id = itemFromTab(tab)
  const wanted = id === null ? '' : `#item-${id}`
  if (location.hash === wanted) return
  if (wanted) {
    location.hash = wanted
  } else {
    history.pushState(history.state, '', location.pathname + location.search)
  }
}

function activate(tab: string): void {
  activeTab.value = tab
  const id = itemFromTab(tab)
  if (id !== null) tracker.value.touch(id)
  if (tabbar && tabbar.getActive() !== tab) tabbar.setActive(tab)
  syncHash(tab)
}

function goHome(): void {
  highlighted.value = null
  activate(HOME_TAB)
}

function openInNewTab(url: string): void {
  window.open(url, '_blank', 'noopener,noreferrer')
}

/** Opens or reuses the tab of an iframe item. */
function openItemTab(item: MenuNode): void {
  const result = tracker.value.openItem(item.id)
  if (result.evicted !== null) {
    const evicted = openTabs.value.find((t) => t.item.id === result.evicted)
    closeTab(result.evicted)
    if (evicted) notify(t('dashboard.tabLimit', { limit: tracker.value.limit, title: evicted.item.title }))
  }
  if (result.created && tabbar) {
    openTabs.value.push({ item, reloads: 0 })
    tabbar.addTab({ id: tabId(item.id), tab: item.title } as never, openTabs.value.length)
    updateClosable()
  }
  activate(tabId(item.id))
}

function closeTab(id: number): void {
  tracker.value.close(id)
  openTabs.value = openTabs.value.filter((t) => t.item.id !== id)
  if (tabbar && tabbar.getCell(tabId(id))) tabbar.removeTab(tabId(id))
  updateClosable()
}

/** Runs the action configured for an item (iframe tab or new browser tab). */
function runItem(item: MenuNode): void {
  if (!item.url) return
  if (item.target === 'blank') {
    openInNewTab(item.url)
    return
  }
  openItemTab(item)
}

function onToolbarClick(id: string | number): void {
  const themeChange = parseThemeId(id)
  if (themeChange) {
    themeStore.set(themeChange)
    return
  }
  const locale = parseLocaleId(id)
  if (locale) {
    localeStore.set(locale)
    return
  }
  const action = parseToolbarId(id)
  switch (action.kind) {
    case 'home':
      goHome()
      return
    case 'admin':
      void router.push('/admin')
      return
    case 'lock':
      // DashboardPage swaps this view for the unlock screen.
      void dashboard.lock()
      return
    case 'open': {
      const item = menuStore.byId.get(action.id)
      if (item) runItem(item)
      return
    }
    case 'item': {
      const item = menuStore.byId.get(action.id)
      // Items with children only open their dropdown.
      if (item && item.items.length === 0) runItem(item)
      return
    }
  }
}

/** Applies the current location hash: activates or opens the referenced tab. */
function applyHash(): void {
  const id = parseHash(location.hash)
  if (id === null) {
    if (activeTab.value !== HOME_TAB) goHome()
    return
  }
  const item = menuStore.byId.get(id)
  if (!item || !item.url) {
    notify(t('dashboard.itemNotFound'))
    goHome()
    return
  }
  if (item.target === 'blank') {
    // Browsers block popups not triggered by a click: highlight instead.
    activate(HOME_TAB)
    highlighted.value = item.id
    return
  }
  openItemTab(item)
}

function reloadTab(tab: OpenTab): void {
  tab.reloads++
}

function isBlockedSameOrigin(item: MenuNode): boolean {
  return !!item.url && isSameOrigin(item.url, location.origin)
}

async function retry(): Promise<void> {
  await menuStore.load(props.slug)
  refreshToolbar()
}

onMounted(async () => {
  // Registered before any await, so onBeforeUnmount always removes them. Coming
  // back to the tab or window checks at once; otherwise every MENU_SYNC_MS.
  document.addEventListener('visibilitychange', onVisibilityChange)
  window.addEventListener('focus', onVisibilityChange)
  menuSyncTimer = setInterval(() => void syncMenu(), MENU_SYNC_MS)

  await configStore.load()
  tracker.value = new TabTracker(configStore.config.max_tabs)

  const layout = own(
    new dhx.Layout(root.value!, {
      type: 'none',
      rows: [
        { id: 'header', height: 'content' },
        { id: 'tabs', height: 'content' },
        { id: 'content', html: '<div class="panes" data-panes></div>' },
      ],
    }),
  )

  toolbar = own(new dhx.Toolbar(DETACHED, { css: 'app-toolbar', navigationType: 'click' }))
  toolbar.events.on('click', onToolbarClick)
  layout.getCell('header').attach(toolbar)

  tabbar = own(
    new dhx.Tabbar(DETACHED, {
      css: 'app-tabbar',
      noContent: true,
      tabWidth: Math.round(TAB_WIDTH * uiScale()),
      tabHeight: Math.round(TAB_HEIGHT * uiScale()),
      tabAlign: 'left',
      closable: [],
      views: [{ id: HOME_TAB, tab: 'Home' }],
    }),
  )
  tabbar.events.on('change', (id: string) => activate(id))
  tabbar.events.on('beforeClose', (id: string) => id !== HOME_TAB)
  tabbar.events.on('afterClose', (id: string) => {
    const itemId = itemFromTab(id)
    if (itemId === null) return
    tracker.value.close(itemId)
    openTabs.value = openTabs.value.filter((t) => t.item.id !== itemId)
    updateClosable()
    if (activeTab.value === id) activate(tabbar!.getActive() || HOME_TAB)
  })
  layout.getCell('tabs').attach(tabbar)
  stopScaleWatch = onScaleChange((scale) => {
    if (!tabbar) return
    tabbar.config.tabWidth = Math.round(TAB_WIDTH * scale)
    tabbar.config.tabHeight = Math.round(TAB_HEIGHT * scale)
    tabbar.paint()
  })

  await dhx.awaitRedraw()
  panesTarget.value = root.value!.querySelector<HTMLElement>('[data-panes]')

  await menuStore.load(props.slug)
  refreshToolbar()
  await nextTick()
  applyHash()
  window.addEventListener('hashchange', applyHash)
})

onBeforeUnmount(() => {
  window.removeEventListener('hashchange', applyHash)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('focus', onVisibilityChange)
  if (menuSyncTimer) clearInterval(menuSyncTimer)
  stopScaleWatch?.()
  toolbar = null
  tabbar = null
})

watch(
  () => [props.title, themeStore.prefs, currentLocale.value],
  () => refreshToolbar(),
)
</script>

<template>
  <div ref="root" class="h-full w-full" data-testid="dashboard"></div>

  <Teleport v-if="panesTarget" :to="panesTarget">
    <HomePane
      v-show="activeTab === HOME_TAB"
      class="pane"
      :slug="slug"
      :tree="menuStore.tree"
      :error="menuStore.error"
      :loading="menuStore.loading"
      :highlighted="highlighted"
      @run="runItem"
      @retry="retry"
      @dragging="draggingCard = $event"
    />

    <div
      v-for="tab in openTabs"
      v-show="activeTab === tabId(tab.item.id)"
      :key="tab.item.id"
      class="pane"
      :data-testid="`pane-${tab.item.id}`"
    >
      <div class="context-bar">
        <i v-if="tab.item.icon" :class="tab.item.icon" aria-hidden="true"></i>
        <span class="truncate font-semibold text-text" data-testid="context-title">{{ tab.item.title }}</span>
        <span class="truncate text-muted">{{ tab.item.url }}</span>
        <span class="flex-1"></span>
        <button
          type="button"
          class="btn btn-primary btn-icon"
          :title="t('dashboard.openInNewTab')"
          :aria-label="t('dashboard.openInNewTab')"
          @click="openInNewTab(tab.item.url!)"
        >
          <i class="mdi mdi-open-in-new" aria-hidden="true"></i>
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-icon"
          :title="t('dashboard.reload')"
          :aria-label="t('dashboard.reload')"
          @click="reloadTab(tab)"
        >
          <i class="mdi mdi-refresh" aria-hidden="true"></i>
        </button>
      </div>

      <div
        v-if="isBlockedSameOrigin(tab.item)"
        class="flex flex-1 flex-col items-center justify-center gap-3 text-text"
      >
        <p>{{ t('dashboard.sameOrigin') }}</p>
      </div>
      <iframe
        v-else
        :key="tab.reloads"
        :src="tab.item.url!"
        :title="tab.item.title"
        :sandbox="sandbox"
        referrerpolicy="no-referrer"
        data-testid="item-iframe"
      ></iframe>
    </div>
  </Teleport>
</template>
