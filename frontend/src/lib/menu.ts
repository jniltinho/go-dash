import type { MenuInput, MenuNode, Move } from '../api/types'
import { t } from './i18n'

/** Toolbar item IDs used by the dashboard. */
export const ids = {
  title: 'title',
  home: 'action-home',
  admin: 'action-admin',
  /** Locks a password-protected dashboard again (ends the viewer session). */
  lock: 'action-lock',
  item: (id: number) => `item-${id}`,
  open: (id: number) => `open-${id}`,
}

/** Parsed toolbar click target. */
export type ToolbarAction =
  | { kind: 'home' }
  | { kind: 'admin' }
  | { kind: 'lock' }
  | { kind: 'item'; id: number }
  | { kind: 'open'; id: number }
  | { kind: 'none' }

/** Resolves a toolbar item ID into an action. */
export function parseToolbarId(id: string | number): ToolbarAction {
  const value = String(id)
  if (value === ids.home) return { kind: 'home' }
  if (value === ids.admin) return { kind: 'admin' }
  if (value === ids.lock) return { kind: 'lock' }
  const match = /^(item|open)-(\d+)$/.exec(value)
  if (!match) return { kind: 'none' }
  return { kind: match[1] as 'item' | 'open', id: Number(match[2]) }
}

/** Parses `#item-<id>` from a location hash. */
export function parseHash(hash: string): number | null {
  const match = /^#item-(\d+)$/.exec(hash)
  return match ? Number(match[1]) : null
}

/** Longest title accepted by the API (internal/menu maxTitleLength). */
export const MAX_TITLE_LENGTH = 120

/**
 * Input for a copy of an item (not its subitems): same parent, URL and
 * settings, title marked as a copy. Positions do not shift siblings, so after
 * creating it, `placeAfterMoves` puts the copy right after the original.
 */
export function duplicateInput(item: MenuNode): MenuInput {
  const suffix = ` (${t('menu.copySuffix')})`
  const base = [...item.title].slice(0, MAX_TITLE_LENGTH - [...suffix].length).join('')
  return {
    parent_id: item.parent_id,
    title: `${base}${suffix}`,
    url: item.url,
    target: item.target,
    placement: item.placement,
    icon: item.icon,
    description: item.description,
    position: item.position + 1,
    enabled: item.enabled,
  }
}

/**
 * Moves that renumber `siblings` (in display order) with `newId` inserted
 * right after `afterId`; `newId` is ignored if already among the siblings.
 */
export function placeAfterMoves(
  siblings: readonly MenuNode[],
  afterId: number,
  newId: number,
  parentId: number | null,
): Move[] {
  const order = siblings.map((s) => s.id).filter((id) => id !== newId)
  const at = order.indexOf(afterId)
  order.splice(at === -1 ? order.length : at + 1, 0, newId)
  return order.map((id, position) => ({ id, parent_id: parentId, position }))
}

/** Flattens a tree into a map by ID. */
export function indexTree(nodes: MenuNode[], into = new Map<number, MenuNode>()): Map<number, MenuNode> {
  for (const node of nodes) {
    into.set(node.id, node)
    indexTree(node.items, into)
  }
  return into
}

/**
 * Minimal toolbar element shape (subset of DHTMLX IToolbarElement). Values are
 * passed as `value`, which DHTMLX renders as text, never as `html`.
 */
export interface ToolbarElement {
  id: string
  type: 'title' | 'navItem' | 'menuItem' | 'separator' | 'spacer'
  value?: string
  icon?: string
  tooltip?: string
  /** Extra CSS class (DHTMLX `css`), e.g. `menu-current` for the active option. */
  css?: string
  items?: ToolbarElement[]
}

function toElement(node: MenuNode, type: 'navItem' | 'menuItem'): ToolbarElement {
  const element: ToolbarElement = { id: ids.item(node.id), type, value: node.title }
  if (node.icon) element.icon = node.icon
  if (node.description) element.tooltip = node.description

  if (node.items.length > 0) {
    const children = node.items.map((child) => toElement(child, 'menuItem'))
    if (node.url) {
      children.unshift(
        {
          id: ids.open(node.id),
          type: 'menuItem',
          value: t('toolbar.open', { title: node.title }),
          icon: 'mdi mdi-open-in-app',
        },
        { id: `sep-${node.id}`, type: 'separator' },
      )
    }
    element.items = children
  }
  return element
}

/** Keeps nodes (recursively) whose placement includes the given surface. */
function filterPlacement(nodes: MenuNode[], hide: 'menu' | 'card'): MenuNode[] {
  return nodes
    .filter((n) => n.placement !== hide)
    .map((n) => ({ ...n, items: filterPlacement(n.items, hide) }))
}

/** Items shown in the header toolbar (placement "menu" or "both"). */
export function menuTree(tree: MenuNode[]): MenuNode[] {
  return filterPlacement(tree, 'card')
}

/** Items shown on the home tab (placement "card" or "both"). */
export function cardTree(tree: MenuNode[]): MenuNode[] {
  return filterPlacement(tree, 'menu')
}

/** Moves the element at index `from` to index `to` (returns a new array). */
export function moveIndex<T>(list: readonly T[], from: number, to: number): T[] {
  const copy = [...list]
  const [item] = copy.splice(from, 1)
  copy.splice(Math.max(0, Math.min(to, copy.length)), 0, item!)
  return copy
}

/**
 * "Lock" action, shown on password-protected dashboards. A fresh object
 * each time: DHTMLX keeps (and mutates) the objects it parses.
 */
export function lockAction(): ToolbarElement {
  return {
    id: ids.lock,
    type: 'navItem',
    value: t('toolbar.lock'),
    icon: 'mdi mdi-lock',
    tooltip: t('toolbar.lockTooltip'),
  }
}

/**
 * Builds the dashboard toolbar: title, one dropdown per root item, spacer,
 * optional extra items (e.g. the theme menu or the lock action) and fixed actions.
 */
export function buildToolbar(
  title: string,
  tree: MenuNode[],
  extra: ToolbarElement[] = [],
): ToolbarElement[] {
  return [
    { id: ids.title, type: 'title', value: title },
    { id: 'sep-title', type: 'separator' },
    ...menuTree(tree).map((node) => toElement(node, 'navItem')),
    { id: 'spacer', type: 'spacer' },
    ...extra,
    { id: ids.home, type: 'navItem', value: 'Home', icon: 'mdi mdi-home' },
    { id: ids.admin, type: 'navItem', value: 'Admin', icon: 'mdi mdi-cog' },
  ]
}

/** Reports whether url has the same origin as the dashboard. */
export function isSameOrigin(url: string, origin: string): boolean {
  try {
    return new URL(url).origin === origin
  } catch {
    return false
  }
}
