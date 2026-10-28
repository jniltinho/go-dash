import type { MenuNode } from '../api/types'
import { basePath, storageKey } from './basePath'

/**
 * Each analyst may arrange the home cards; the order is a per-browser
 * convenience kept in localStorage (never sent to the server), one key per
 * dashboard. The default order is the menu order set in the admin.
 */
export const CARD_ORDER_PREFIX = 'dashboard.cardOrder'

/** Key used before multiple dashboards existed (it belonged to the default one). */
export const LEGACY_CARD_ORDER_KEY = CARD_ORDER_PREFIX

/** Storage key of a dashboard's card order ("default" for the dashboard at "/"). */
export function cardOrderKey(slug: string, base = basePath()): string {
  return storageKey(`${CARD_ORDER_PREFIX}.${slug}`, base)
}

type OrderStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

/**
 * Moves the legacy key to the default dashboard's key once. An order already
 * saved under the new key wins. Only the instance at "/" owns the legacy key:
 * one under a prefix never reads or deletes it.
 */
export function migrateLegacyCardOrder(storage: OrderStorage, base = basePath()): void {
  if (base !== '/') return
  const legacy = storage.getItem(LEGACY_CARD_ORDER_KEY)
  if (legacy === null) return
  const target = cardOrderKey('default', base)
  if (storage.getItem(target) === null) storage.setItem(target, legacy)
  storage.removeItem(LEGACY_CARD_ORDER_KEY)
}

/**
 * Cards in the saved order. Saved IDs that no longer exist are ignored; cards
 * missing from the saved order (e.g. added later by an admin) follow, in the
 * default order.
 */
export function applyCardOrder(cards: readonly MenuNode[], saved: readonly number[] | null): MenuNode[] {
  if (!saved?.length) return [...cards]
  const byId = new Map(cards.map((c) => [c.id, c]))
  const ordered = [...new Set(saved)].flatMap((id) => byId.get(id) ?? [])
  const placed = new Set(ordered.map((c) => c.id))
  return [...ordered, ...cards.filter((c) => !placed.has(c.id))]
}

/** Parses a stored order; anything malformed counts as "no saved order". */
export function parseCardOrder(raw: string | null): number[] | null {
  if (!raw) return null
  try {
    const value: unknown = JSON.parse(raw)
    if (!Array.isArray(value) || !value.every((id) => Number.isSafeInteger(id) && id > 0)) return null
    return value.length ? (value as number[]) : null
  } catch {
    return null
  }
}

/** Storage may be unavailable (private mode, blocked): the default order is used. */
export function loadCardOrder(slug: string): number[] | null {
  try {
    migrateLegacyCardOrder(localStorage)
    return parseCardOrder(localStorage.getItem(cardOrderKey(slug)))
  } catch {
    return null
  }
}

export function saveCardOrder(slug: string, ids: readonly number[]): void {
  try {
    localStorage.setItem(cardOrderKey(slug), JSON.stringify(ids))
  } catch {
    // Arranging still works for this page view.
  }
}
