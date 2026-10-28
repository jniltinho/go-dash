import { defineStore } from 'pinia'
import { computed, ref, shallowRef } from 'vue'

import { api, ApiError, fetchConfigWithin, setCsrfToken } from '../api/client'
import type { DashboardStatus, MenuNode, PublicConfig, SessionStatus } from '../api/types'
import { classifyMenuError, DEFAULT_SLUG } from '../lib/dashboard'
import { applyLocale, currentLocale, type Locale, type MessageKey, saveLocale } from '../lib/i18n'
import { indexTree } from '../lib/menu'
import { applyTheme, DEFAULT_THEME, saveThemePrefs, type ThemePrefs } from '../lib/theme'

/** Never block rendering on /api/config for longer than this. */
export const CONFIG_TIMEOUT_MS = 1500

const defaultConfig: PublicConfig = {
  title: 'Go Dash',
  max_tabs: 10,
  iframe_sandbox: 'allow-scripts allow-same-origin allow-forms allow-popups',
  public_url: '',
  default_theme: 'ocean-light',
  default_theme_style: 'flat',
  build: { version: 'dev', commit: '', build_date: '' },
}

/** Public configuration delivered by the server. */
export const useConfigStore = defineStore('config', () => {
  const config = ref<PublicConfig>(defaultConfig)
  const loaded = ref(false)

  /** Stores a config fetched elsewhere (main.ts fetches it once at startup). */
  function set(value: PublicConfig): void {
    config.value = value
    document.title = value.title
    loaded.value = true
  }

  async function load(): Promise<void> {
    if (loaded.value) return
    // Keep defaults when the config is slow or unavailable: the dashboard still works.
    const value = await fetchConfigWithin(CONFIG_TIMEOUT_MS)
    if (value) set(value)
    loaded.value = true
  }

  return { config, load, set }
})

/**
 * Public menu tree of the current dashboard. The store is shared, so every
 * result is tied to the slug and generation that requested it: switching
 * dashboards, locking or losing access resets the tree and drops late
 * responses, so links of one dashboard never show up on another.
 */
export const useMenuStore = defineStore('menu', () => {
  const slug = ref<string | null>(null)
  const tree = shallowRef<MenuNode[]>([])
  const byId = shallowRef(new Map<number, MenuNode>())
  /** Message key of the last load failure (translated when shown). */
  const error = ref<MessageKey | null>(null)
  const loading = ref(false)
  /** Set when the server refuses the menu: 401 (locked) or 404 (dashboard gone). */
  const access = ref<'ok' | 'locked' | 'not-found'>('ok')
  /** Bumped by every load, refresh and reset: only the latest request may apply its result. */
  let generation = 0

  /** Forgets everything about the previous dashboard. */
  function reset(next: string | null = null): void {
    generation++
    slug.value = next
    tree.value = []
    byId.value = new Map()
    error.value = null
    loading.value = false
    access.value = 'ok'
  }

  /** Applies a refused request (locked or deleted dashboard). Returns true if it was one. */
  function denied(err: unknown, forSlug: string): boolean {
    const failure = classifyMenuError(err)
    if (failure === 'transient') return false
    reset(forSlug)
    access.value = failure
    return true
  }

  async function load(forSlug: string): Promise<void> {
    if (slug.value !== forSlug) reset(forSlug)
    const ticket = ++generation
    loading.value = true
    error.value = null
    try {
      const next = await api.menu(forSlug)
      if (ticket !== generation) return
      tree.value = next
      byId.value = indexTree(next)
      access.value = 'ok'
    } catch (err) {
      if (ticket !== generation) return
      if (denied(err, forSlug)) return
      tree.value = []
      byId.value = new Map()
      error.value = 'home.menuLoadFailed'
    } finally {
      // Only the latest load ends the loading state.
      if (ticket === generation) loading.value = false
    }
  }

  /**
   * Background reload (changes made by an admin elsewhere). Keeps the current
   * menu on network or server errors, but a refused request (401/404) clears
   * it. Drops the result if a newer load/refresh/reset happened or `canApply`
   * says no (checked when the response arrives), and returns true only when
   * the menu actually changed.
   */
  async function refresh(canApply: () => boolean = () => true): Promise<boolean> {
    const forSlug = slug.value
    if (loading.value || forSlug === null) return false
    const ticket = ++generation
    try {
      const next = await api.menu(forSlug)
      if (ticket !== generation || !canApply()) return false
      if (JSON.stringify(next) === JSON.stringify(tree.value)) return false
      tree.value = next
      byId.value = indexTree(next)
      error.value = null
      return true
    } catch (err) {
      if (ticket === generation) denied(err, forSlug)
      return false
    }
  }

  return { slug, tree, byId, error, loading, access, load, refresh, reset }
})

/** Where the current dashboard stands before its menu can be shown. */
export type DashboardState = 'loading' | 'ready' | 'locked' | 'not-found' | 'error'

/** Status of the dashboard being viewed (title, password protection). */
export const useDashboardStore = defineStore('dashboard', () => {
  const slug = ref<string>(DEFAULT_SLUG)
  const status = ref<DashboardStatus | null>(null)
  const state = ref<DashboardState>('loading')
  let generation = 0

  async function open(forSlug: string): Promise<void> {
    const ticket = ++generation
    slug.value = forSlug
    status.value = null
    state.value = 'loading'
    try {
      const next = await api.dashboardStatus(forSlug)
      if (ticket !== generation) return
      status.value = next
      state.value = next.protected && !next.unlocked ? 'locked' : 'ready'
    } catch (err) {
      if (ticket !== generation) return
      state.value = err instanceof ApiError && err.status === 404 ? 'not-found' : 'error'
    }
  }

  /** Throws ApiError on a wrong password (401) or rate limit (429). */
  async function unlock(password: string): Promise<void> {
    await api.unlock(slug.value, password)
    await open(slug.value)
  }

  async function lock(): Promise<void> {
    // Forget the protected menu before the unlock screen shows up.
    useMenuStore().reset(slug.value)
    state.value = 'loading'
    try {
      await api.lock(slug.value)
    } finally {
      await open(slug.value)
    }
  }

  /** The menu was refused while viewing (password changed or dashboard deleted). */
  function accessLost(reason: 'locked' | 'not-found'): void {
    generation++
    state.value = reason
  }

  return { slug, status, state, open, unlock, lock, accessLost }
})

/** Admin session state. */
export const useAuthStore = defineStore('auth', () => {
  const status = ref<SessionStatus>({ admin_enabled: false, authenticated: false, csrf_token: null })

  async function refresh(): Promise<SessionStatus> {
    status.value = await api.session()
    setCsrfToken(status.value.csrf_token)
    return status.value
  }

  async function login(password: string): Promise<void> {
    await api.login(password)
    await refresh()
  }

  async function logout(): Promise<void> {
    try {
      await api.logout()
    } finally {
      setCsrfToken(null)
      status.value = { ...status.value, authenticated: false, csrf_token: null }
    }
  }

  return { status, refresh, login, logout }
})

/**
 * Viewer theme preference (theme + flat/3D style). Starts from the viewer's
 * saved choice or the server default (see main.ts); only explicit choices are
 * persisted in localStorage.
 */
export const useThemeStore = defineStore('theme', () => {
  const prefs = ref<ThemePrefs>(DEFAULT_THEME)

  /** Sets the initial theme without saving it as the viewer's choice. */
  function init(initial: ThemePrefs): void {
    prefs.value = initial
  }

  function set(change: Partial<ThemePrefs>): void {
    prefs.value = { ...prefs.value, ...change }
    applyTheme(prefs.value)
    saveThemePrefs(prefs.value)
  }

  return { prefs, init, set }
})

/**
 * UI language. Starts from the viewer's saved choice or the browser language
 * (see main.ts); only explicit choices are persisted in localStorage.
 */
export const useLocaleStore = defineStore('locale', () => {
  const locale = computed(() => currentLocale.value)

  function set(next: Locale): void {
    applyLocale(next)
    saveLocale(next)
  }

  return { locale, set }
})
