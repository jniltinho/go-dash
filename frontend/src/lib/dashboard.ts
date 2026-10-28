import { ApiError } from '../api/client'
import type { Dashboard } from '../api/types'

/** API sentinel for the default dashboard (served at "/"). */
export const DEFAULT_SLUG = 'default'

/** Browser path of a dashboard: "/" for the default one, "/dash/<slug>" otherwise. */
export function dashboardPath(slug: string | null): string {
  return !slug || slug === DEFAULT_SLUG ? '/' : `/dash/${slug}`
}

/** Slug rule of the server (internal/menu ValidSlugSyntax), reserved names aside. */
export const SLUG_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$/

/** Reserved slugs (internal/menu reservedSlugs). */
export const RESERVED_SLUGS: ReadonlySet<string> = new Set([
  'admin',
  'api',
  'assets',
  'dash',
  DEFAULT_SLUG,
  'healthz',
  'login',
])

/** Lower-cases and trims what the admin typed, as the server does. */
export function normalizeSlug(raw: string): string {
  return raw.trim().toLowerCase()
}

/** Why the menu of a dashboard is unavailable; "transient" keeps the last menu. */
export type MenuFailure = 'locked' | 'not-found' | 'transient'

/** Classifies a menu request error: 401 locks, 404 means the dashboard is gone. */
export function classifyMenuError(err: unknown): MenuFailure {
  if (err instanceof ApiError) {
    if (err.status === 401) return 'locked'
    if (err.status === 404) return 'not-found'
  }
  return 'transient'
}

/** Label of a dashboard in the admin, e.g. "Acme (/dash/acme)". */
export function dashboardLabel(d: Pick<Dashboard, 'slug' | 'title'>, configTitle: string): string {
  const title = d.title || configTitle
  return `${title} (${dashboardPath(d.slug)})`
}
