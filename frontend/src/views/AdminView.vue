<script setup lang="ts">
import * as dhx from '@dhtmlx/suite'
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'

import { api, ApiError } from '../api/client'
import type {
  Dashboard,
  DashboardRef,
  ImportMode,
  MenuInput,
  MenuNode,
  Move,
  Placement,
  Target,
} from '../api/types'
import DashboardsPanel from '../components/DashboardsPanel.vue'
import IconPicker from '../components/IconPicker.vue'
import { DETACHED, useDhx } from '../composables/useDhx'
import { dashboardLabel, dashboardPath, DEFAULT_SLUG } from '../lib/dashboard'
import { currentLocale, localeMenu, parseLocaleId, t } from '../lib/i18n'
import { duplicateInput, indexTree, placeAfterMoves, type ToolbarElement } from '../lib/menu'
import { translateError } from '../lib/messages'
import { modalPosition } from '../lib/modal'
import { onScaleChange, uiScale } from '../lib/scale'
import { parseThemeId, themeMenu } from '../lib/theme'
import { useAuthStore, useConfigStore, useLocaleStore, useThemeStore } from '../stores/app'

interface FormValues {
  title: string
  url: string
  target: Target
  placement: Placement
  icon: string
  description: string
  position: string | number
  enabled: boolean
}

const FIELDS = ['title', 'url', 'target', 'placement', 'icon', 'description', 'position', 'enabled'] as const

/** Short suffix shown in the tree for items not shown everywhere. */
function placementSuffix(placement: Placement): string {
  if (placement === 'menu') return ` · ${t('admin.menuOnly')}`
  if (placement === 'card') return ` · ${t('admin.cardOnly')}`
  return ''
}

const router = useRouter()
const auth = useAuthStore()
const configStore = useConfigStore()
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { own, release } = useDhx()
const root = ref<HTMLElement | null>(null)
/** Value of the icon field when the picker opened; null = picker closed. */
const pickerValue = ref<string | null | undefined>(undefined)
/** Dashboards panel (create, rename, delete, passwords) is open. */
const managing = ref(false)
const dashboards = shallowRef<Dashboard[]>([])
/** Dashboard whose items are being edited. */
const current = shallowRef<Dashboard | null>(null)
/** Admin URL reference of the current dashboard. */
const dashRef = (): DashboardRef => current.value?.id ?? DEFAULT_SLUG

let tree: dhx.Tree
let form: dhx.Form
let toolbar: dhx.Toolbar
let byId = new Map<number, MenuNode>()
/** Last tree loaded, kept to re-render it in another language. */
let treeNodes: MenuNode[] = []
/** Builds the form's context line; rebuilt when the language changes. */
let contextText: () => string = () => ''
/** Tree column width at scale 1 (Full HD). */
const TREE_WIDTH = 340
let stopScaleWatch: (() => void) | null = null

onBeforeUnmount(() => stopScaleWatch?.())
/** Item being edited; null = creating a new item. */
let editingId: number | null = null
/** Parent of the item being created. */
let newParentId: number | null = null
/** Copy being created by "Duplicate": its original and suggested position. */
let duplicateOf: { id: number; position: number } | null = null

function notify(text: string): void {
  dhx.message({ text, expire: 4000 })
}

function toTreeData(nodes: MenuNode[]): unknown[] {
  return nodes.map((n) => ({
    id: String(n.id),
    value: `${n.title}${placementSuffix(n.placement)}${n.enabled ? '' : ` (${t('admin.disabled')})`}`,
    icon: n.icon ? { folder: n.icon, openFolder: n.icon, file: n.icon } : undefined,
    opened: true,
    items: n.items.length ? toTreeData(n.items) : undefined,
  }))
}

/** Bumped by every tree load: a late response for another dashboard is dropped. */
let treeGeneration = 0

/** Empties the tree and form, e.g. while switching dashboards. */
function clearTree(): void {
  treeGeneration++
  byId = new Map()
  treeNodes = []
  tree.data.removeAll()
}

/** Re-renders the loaded tree (e.g. after a language change), keeping the selection. */
function renderTree(): void {
  const selected = tree.selection.getId()
  tree.data.removeAll()
  tree.data.parse(toTreeData(treeNodes) as never)
  if (selected !== undefined && selected !== null && tree.data.exists(selected)) tree.selection.add(selected)
}

async function loadTree(selectId?: number): Promise<void> {
  const ticket = ++treeGeneration
  const nodes = await api.adminTree(dashRef())
  if (ticket !== treeGeneration) return
  byId = indexTree(nodes)
  treeNodes = nodes
  tree.data.removeAll()
  tree.data.parse(toTreeData(nodes) as never)
  if (selectId !== undefined && byId.has(selectId)) {
    tree.selection.add(String(selectId))
    edit(selectId)
  }
}

function clearErrors(): void {
  for (const name of FIELDS) {
    const field = form.getItem(name)
    field.config.validation = undefined
    field.clearValidate?.()
  }
}

function showFieldError(field: string | undefined, message: string): void {
  const control = field && (FIELDS as readonly string[]).includes(field) ? form.getItem(field) : null
  if (!control) {
    notify(message)
    return
  }
  control.setProperties({ errorMessage: message })
  control.config.validation = () => false
  control.validate()
}

/** Shows the chosen icon inside the icon field. */
function previewIcon(value: string): void {
  form.getItem('icon').setProperties({ icon: value.trim() })
}

function openIconPicker(): void {
  pickerValue.value = (form.getItem('icon').getValue() as string) || null
}

function onIconSelected(value: string | null): void {
  form.getItem('icon').setValue(value ?? '')
  previewIcon(value ?? '')
  pickerValue.value = undefined
}

function edit(id: number | null, parentId: number | null = null): void {
  clearErrors()
  duplicateOf = null
  editingId = id
  newParentId = parentId
  const item = id === null ? null : byId.get(id)
  const parentTitle = (id === null ? parentId : item?.parent_id) ?? null
  const parent = parentTitle ? (byId.get(parentTitle)?.title ?? '') : null
  setContext(() => {
    if (id === null) return parent === null ? t('admin.newItemRoot') : t('admin.newItemIn', { parent })
    return parent === null ? t('admin.editing', { id }) : t('admin.editingIn', { id, parent })
  })
  form.setValue({
    title: item?.title ?? '',
    url: item?.url ?? '',
    target: item?.target ?? 'iframe',
    placement: item?.placement ?? 'both',
    icon: item?.icon ?? '',
    description: item?.description ?? '',
    position: item?.position ?? 0,
    enabled: item?.enabled ?? true,
  })
  previewIcon(item?.icon ?? '')
}

/** Fills the form with a copy of the edited item; nothing is saved until "Save". */
function duplicate(): void {
  const item = editingId === null ? undefined : byId.get(editingId)
  if (!item) {
    notify(t('admin.selectToDuplicate'))
    return
  }
  clearErrors()
  const copy = duplicateInput(item)
  editingId = null
  newParentId = copy.parent_id
  duplicateOf = { id: item.id, position: copy.position }
  tree.selection.remove()
  const parentTitle = copy.parent_id === null ? null : byId.get(copy.parent_id)?.title
  setContext(() =>
    parentTitle
      ? t('admin.copyOfIn', { title: item.title, parent: parentTitle })
      : t('admin.copyOf', { title: item.title }),
  )
  form.setValue({
    title: copy.title,
    url: copy.url ?? '',
    target: copy.target,
    placement: copy.placement,
    icon: copy.icon ?? '',
    description: copy.description ?? '',
    position: copy.position,
    enabled: copy.enabled,
  })
  previewIcon(copy.icon ?? '')
  form.getItem('title').focus()
}

/** Copies the URL field to the clipboard (with a fallback for plain-HTTP origins). */
async function copyUrl(): Promise<void> {
  const url = String(form.getItem('url').getValue() ?? '').trim()
  if (!url) {
    notify(t('admin.noUrl'))
    return
  }
  try {
    await navigator.clipboard.writeText(url)
  } catch {
    // The async clipboard API needs a secure context (HTTPS or localhost).
    const area = document.createElement('textarea')
    area.value = url
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.append(area)
    area.select()
    const ok = document.execCommand('copy')
    area.remove()
    if (!ok) {
      notify(t('admin.copyFailed'))
      return
    }
  }
  notify(t('admin.urlCopied'))
}

function readInput(): MenuInput {
  const v = form.getValue() as FormValues
  const text = (s: string) => (s && s.trim() ? s.trim() : null)
  const parent = editingId === null ? newParentId : (byId.get(editingId)?.parent_id ?? null)
  return {
    parent_id: parent,
    title: v.title ?? '',
    url: text(v.url),
    target: v.target,
    placement: v.placement,
    icon: text(v.icon),
    description: text(v.description),
    position: Number(v.position) || 0,
    enabled: !!v.enabled,
  }
}

async function save(): Promise<void> {
  clearErrors()
  try {
    const input = readInput()
    const item =
      editingId === null
        ? await api.createItem(dashRef(), input)
        : await api.updateItem(dashRef(), editingId, input)
    const copyOf = duplicateOf
    duplicateOf = null
    // A position typed by the admin wins over "right after the original".
    if (copyOf && input.position === copyOf.position) {
      // The copy is already saved: a failed reorder must not block reloading.
      await placeCopy(item.id, copyOf.id, input.parent_id).catch(handleError)
    }
    notify(t('admin.itemSaved'))
    await loadTree(item.id)
  } catch (err) {
    handleError(err)
  }
}

/** Puts a saved copy right after its original (creation does not shift siblings). */
async function placeCopy(copyId: number, originalId: number, parentId: number | null): Promise<void> {
  const fresh = await api.adminTree(dashRef())
  const siblings = parentId === null ? fresh : (indexTree(fresh).get(parentId)?.items ?? [])
  // Only when the original is still a sibling (the parent may have changed meanwhile).
  if (siblings.some((s) => s.id === originalId)) {
    await api.reorder(dashRef(), placeAfterMoves(siblings, originalId, copyId, parentId))
  }
}

async function remove(): Promise<void> {
  if (editingId === null) return
  const item = byId.get(editingId)
  const children = item?.items.length ?? 0
  const ok = await dhx.confirm({
    // Titles are user data: render them as text, never as HTML.
    htmlEnable: false,
    header: t('admin.deleteItem'),
    text: children
      ? t('admin.deleteWithChildren', { title: item?.title ?? '', count: children })
      : t('common.deleteConfirm', { title: item?.title ?? '' }),
    buttons: [t('common.cancel'), t('common.delete')],
  })
  if (!ok) return
  try {
    await api.deleteItem(dashRef(), editingId)
    notify(t('admin.itemDeleted'))
    edit(null)
    await loadTree()
  } catch (err) {
    handleError(err)
  }
}

/** Sends the whole hierarchy after a drag-and-drop, as one atomic batch. */
async function persistOrder(): Promise<void> {
  const moves: Move[] = []
  const rootId = tree.data.getRoot()
  const walk = (parent: string | number) => {
    tree.data.getItems(parent).forEach((child, index) => {
      moves.push({
        id: Number(child.id),
        parent_id: parent === rootId ? null : Number(parent),
        position: index,
      })
      walk(child.id!)
    })
  }
  walk(rootId)
  try {
    await api.reorder(dashRef(), moves)
    notify(t('admin.orderSaved'))
    await loadTree(editingId ?? undefined)
  } catch (err) {
    handleError(err)
    await loadTree()
  }
}

async function exportMenu(): Promise<void> {
  try {
    const doc = await api.exportMenu(dashRef())
    const blob = new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `dashboard-menu-${current.value?.slug ?? DEFAULT_SLUG}.json`
    link.click()
    URL.revokeObjectURL(link.href)
  } catch (err) {
    handleError(err)
  }
}

const IMPORT_WIDTH = 560
const IMPORT_HEIGHT = 520

function openImport(): void {
  const win = own(
    new dhx.Window({
      title: t('admin.importTitle', { dashboard: currentLabel() }),
      modal: true,
      width: IMPORT_WIDTH,
      height: IMPORT_HEIGHT,
      closable: true,
    }),
  )
  const importForm = own(
    new dhx.Form(DETACHED, {
      padding: 16,
      rows: [
        { type: 'text', name: 'help', value: t('admin.importHelp') },
        { type: 'textarea', name: 'json', label: 'JSON', height: 260, required: true },
        {
          type: 'radioGroup',
          name: 'mode',
          label: t('admin.importMode'),
          value: 'append',
          options: {
            cols: [
              { type: 'radioButton', text: t('admin.importAppend'), value: 'append' },
              { type: 'radioButton', text: t('admin.importReplace'), value: 'replace' },
            ],
          },
        },
        {
          align: 'end',
          cols: [
            { type: 'button', name: 'cancel', text: t('common.cancel'), view: 'link' },
            { type: 'button', name: 'import', text: t('admin.import'), submit: true },
          ],
        },
      ],
    }),
  )
  win.attach(importForm)
  let closed = false
  const close = () => {
    if (closed) return
    closed = true
    release(importForm)
    release(win)
  }
  // The header X only hides the window: dispose of it so nothing accumulates.
  win.events.on('afterHide', () => setTimeout(close))
  importForm.getItem('cancel').events.on('click', close)
  importForm.getItem('import').events.on('click', async () => {
    const values = importForm.getValue() as { json: string; mode: ImportMode }
    let doc: unknown
    try {
      doc = JSON.parse(values.json)
    } catch {
      notify(t('admin.invalidJson'))
      return
    }
    try {
      const res = await api.importMenu(dashRef(), doc, values.mode)
      notify(t('admin.imported', { count: res.imported }))
      close()
      edit(null)
      await loadTree()
    } catch (err) {
      handleError(err)
    }
  })
  // Like every modal, 10% below the top (DHTMLX centers windows by default).
  const { left, top } = modalPosition(IMPORT_WIDTH, IMPORT_HEIGHT, {
    width: window.innerWidth,
    height: window.innerHeight,
  })
  win.show(left, top)
}

async function logout(): Promise<void> {
  await auth.logout()
  await router.replace('/admin/login')
}

function currentLabel(): string {
  return current.value ? dashboardLabel(current.value, configStore.config.title) : 'Dashboard'
}

/** Dashboard dropdown: every dashboard (text only) plus the management panel. */
function dashboardMenu(): ToolbarElement {
  return {
    id: 'dash-menu',
    type: 'navItem',
    value: `Dashboard: ${currentLabel()}`,
    icon: 'mdi mdi-view-dashboard-variant',
    items: [
      ...dashboards.value.map((d): ToolbarElement => ({
        id: `dash-${d.id}`,
        type: 'menuItem',
        value: dashboardLabel(d, configStore.config.title),
        icon: d.protected ? 'mdi mdi-lock' : 'mdi mdi-earth',
        css: d.id === current.value?.id ? 'menu-current' : undefined,
      })),
      { id: 'sep-dash', type: 'separator' },
      { id: 'dash-manage', type: 'menuItem', value: t('admin.manageDashboards'), icon: 'mdi mdi-cog' },
    ],
  }
}

function toolbarItems(): ToolbarElement[] {
  return [
    { type: 'title', id: 'title', value: 'Admin' },
    { type: 'separator', id: 'sep0' },
    dashboardMenu(),
    { type: 'separator', id: 'sep1' },
    { type: 'navItem', id: 'new-root', value: t('admin.new'), icon: 'mdi mdi-plus' },
    { type: 'navItem', id: 'new-child', value: t('admin.newChild'), icon: 'mdi mdi-file-tree' },
    { type: 'navItem', id: 'duplicate', value: t('admin.duplicate'), icon: 'mdi mdi-content-duplicate' },
    { type: 'navItem', id: 'save', value: t('common.save'), icon: 'mdi mdi-content-save' },
    { type: 'navItem', id: 'delete', value: t('common.delete'), icon: 'mdi mdi-delete' },
    { type: 'separator', id: 'sep2' },
    { type: 'navItem', id: 'import', value: t('admin.import'), icon: 'mdi mdi-upload' },
    { type: 'navItem', id: 'export', value: t('admin.export'), icon: 'mdi mdi-download' },
    { type: 'spacer', id: 'spacer' },
    themeMenu(themeStore.prefs),
    localeMenu(),
    { type: 'navItem', id: 'dashboard', value: t('admin.viewDashboard'), icon: 'mdi mdi-view-dashboard' },
    { type: 'navItem', id: 'logout', value: t('admin.logout'), icon: 'mdi mdi-logout' },
  ]
}

function repaintToolbar(): void {
  toolbar.data.removeAll()
  toolbar.data.parse(toolbarItems() as never)
}

/** Reloads the dashboard list, keeping the current one when it still exists. */
async function loadDashboards(): Promise<void> {
  dashboards.value = await api.listDashboards()
  const keep = dashboards.value.find((d) => d.id === current.value?.id)
  current.value = keep ?? dashboards.value[0] ?? null
  repaintToolbar()
}

async function selectDashboard(id: number): Promise<void> {
  const next = dashboards.value.find((d) => d.id === id)
  if (!next || next.id === current.value?.id) return
  current.value = next
  repaintToolbar()
  tree.selection.remove()
  clearTree()
  edit(null)
  try {
    await loadTree()
  } catch (err) {
    handleError(err)
  }
}

/** The panel changed dashboards: refresh the list and the tree (the current one may be gone). */
async function onDashboardsChanged(): Promise<void> {
  const before = current.value?.id
  try {
    await loadDashboards()
    if (current.value?.id !== before) {
      // The edited dashboard is gone (deleted): never keep showing its tree.
      tree.selection.remove()
      clearTree()
      edit(null)
    }
    await loadTree()
  } catch (err) {
    handleError(err)
  }
}

function handleError(err: unknown): void {
  if (err instanceof ApiError) {
    if (err.status === 401) {
      void router.replace('/admin/login')
      return
    }
    if (err.status === 400) {
      showFieldError(err.field, translateError(err.message))
      return
    }
    notify(translateError(err.message))
    return
  }
  notify(t('common.unexpectedError'))
}

function setContext(text: () => string): void {
  contextText = text
  form.getItem('context').setValue(text())
}

function targetOptions() {
  return [
    { value: 'iframe', content: t('admin.targetIframe') },
    { value: 'blank', content: t('admin.targetBlank') },
  ]
}

function placementOptions() {
  return [
    { value: 'both', content: t('admin.placementBoth') },
    { value: 'menu', content: t('admin.placementMenu') },
    { value: 'card', content: t('admin.placementCard') },
  ]
}

let layout: dhx.Layout

/** Applies the current language to every DHTMLX widget, keeping the form values. */
function relabel(): void {
  repaintToolbar()
  layout.getCell('tree').config.header = t('admin.treeHeader')
  layout.getCell('form').config.header = t('admin.formHeader')
  layout.paint()
  renderTree()
  const values = form.getValue() as FormValues
  const labels: Record<string, string> = {
    title: t('admin.fieldTitle'),
    url: t('admin.fieldUrl'),
    target: t('admin.fieldTarget'),
    placement: t('admin.fieldPlacement'),
    icon: t('admin.fieldIcon'),
    description: t('admin.fieldDescription'),
    position: t('admin.fieldPosition'),
  }
  for (const [name, label] of Object.entries(labels)) form.getItem(name).setProperties({ label })
  form.getItem('target').setOptions(targetOptions())
  form.getItem('placement').setOptions(placementOptions())
  form.getItem('target').setValue(values.target)
  form.getItem('placement').setValue(values.placement)
  form.getItem('enabled').setProperties({ text: t('admin.fieldEnabled') })
  form.getItem('copyUrl').setProperties({ text: t('admin.copy') })
  form.getItem('pickIcon').setProperties({ text: t('admin.pickIcon') })
  form.getItem('context').setValue(contextText())
}

watch(currentLocale, () => {
  if (layout) relabel()
})

onMounted(async () => {
  layout = own(
    new dhx.Layout(root.value!, {
      type: 'none',
      rows: [
        { id: 'header', height: 'content' },
        {
          cols: [
            {
              id: 'tree',
              width: Math.round(TREE_WIDTH * uiScale()),
              minWidth: 220,
              resizable: true,
              header: t('admin.treeHeader'),
            },
            { id: 'form', header: t('admin.formHeader'), css: 'admin-form-cell' },
          ],
        },
      ],
    }),
  )

  toolbar = own(new dhx.Toolbar(DETACHED, { css: 'app-toolbar', data: toolbarItems() }))
  toolbar.events.on('click', (id: string | number) => {
    const themeChange = parseThemeId(id)
    if (themeChange) {
      themeStore.set(themeChange)
      repaintToolbar()
      return
    }
    const locale = parseLocaleId(id)
    if (locale) {
      localeStore.set(locale)
      return
    }
    const dashMatch = /^dash-(\d+)$/.exec(String(id))
    if (dashMatch) return void selectDashboard(Number(dashMatch[1]))
    switch (id) {
      case 'dash-manage':
        managing.value = true
        return
      case 'new-root':
        tree.selection.remove()
        return edit(null, null)
      case 'new-child':
        return edit(null, editingId)
      case 'duplicate':
        return duplicate()
      case 'save':
        return void save()
      case 'delete':
        return void remove()
      case 'import':
        return openImport()
      case 'export':
        return void exportMenu()
      case 'dashboard':
        return void router.push(dashboardPath(current.value?.slug ?? null))
      case 'logout':
        return void logout()
    }
  })
  stopScaleWatch = onScaleChange((scale) => {
    layout.getCell('tree').config.width = Math.round(TREE_WIDTH * scale)
    layout.paint()
  })
  layout.getCell('header').attach(toolbar)

  tree = own(new dhx.Tree(DETACHED, { dragMode: 'both', selection: true, keyNavigation: true }))
  tree.events.on('itemClick', (id: string | number) => edit(Number(id)))
  tree.events.on('afterDrop', () => void persistOrder())
  layout.getCell('tree').attach(tree)

  form = own(
    new dhx.Form(DETACHED, {
      padding: 16,
      rows: [
        { type: 'text', name: 'context', value: '' },
        { type: 'input', name: 'title', label: t('admin.fieldTitle'), required: true, maxlength: 120 },
        {
          css: 'field-action-row',
          cols: [
            {
              type: 'input',
              name: 'url',
              label: t('admin.fieldUrl'),
              placeholder: 'https://',
            },
            {
              type: 'button',
              name: 'copyUrl',
              text: t('admin.copy'),
              icon: 'mdi mdi-content-copy',
              view: 'flat',
              css: 'field-action-button',
            },
          ],
        },
        {
          type: 'select',
          name: 'target',
          label: t('admin.fieldTarget'),
          value: 'iframe',
          options: targetOptions(),
        },
        {
          type: 'select',
          name: 'placement',
          label: t('admin.fieldPlacement'),
          value: 'both',
          options: placementOptions(),
        },
        {
          css: 'field-action-row',
          cols: [
            { type: 'input', name: 'icon', label: t('admin.fieldIcon'), placeholder: 'mdi mdi-chart-line' },
            {
              type: 'button',
              name: 'pickIcon',
              text: t('admin.pickIcon'),
              icon: 'mdi mdi-emoticon-outline',
              view: 'flat',
              css: 'field-action-button',
            },
          ],
        },
        { type: 'textarea', name: 'description', label: t('admin.fieldDescription') },
        { type: 'input', name: 'position', label: t('admin.fieldPosition'), inputType: 'number', value: 0 },
        { type: 'checkbox', name: 'enabled', text: t('admin.fieldEnabled'), checked: true },
      ],
    }),
  )
  form.events.on('change', (name: string, value: unknown) => {
    clearErrors()
    if (name === 'icon') previewIcon(String(value ?? ''))
  })
  form.getItem('pickIcon').events.on('click', openIconPicker)
  form.getItem('copyUrl').events.on('click', () => void copyUrl())
  layout.getCell('form').attach(form)

  edit(null)
  try {
    await loadDashboards()
    await loadTree()
  } catch (err) {
    handleError(err)
  }
})
</script>

<template>
  <div ref="root" class="h-full w-full" data-testid="admin"></div>
  <DashboardsPanel
    v-if="managing"
    :dashboards="dashboards"
    :config-title="configStore.config.title"
    @changed="onDashboardsChanged"
    @close="managing = false"
  />
  <IconPicker
    v-if="pickerValue !== undefined"
    :current="pickerValue"
    @select="onIconSelected"
    @close="pickerValue = undefined"
  />
</template>
