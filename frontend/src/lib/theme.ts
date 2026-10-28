import { findTheme, THEME_GROUPS, THEMES, type ThemeDefinition, type ThemeGradients } from '../theme/registry'
import { storageKey } from './basePath'
import { currentLocale, t, type Locale } from './i18n'
import type { ToolbarElement } from './menu'

export type ThemeName = string
export type ThemeStyle = 'flat' | '3d'

export interface ThemePrefs {
  name: ThemeName
  style: ThemeStyle
}

/** Used when neither a saved choice nor the server default is available. */
export const DEFAULT_THEME: ThemePrefs = { name: 'ocean-light', style: 'flat' }

const STORAGE_NAME = 'dashboard.theme'

export const styles: ReadonlyArray<{ style: ThemeStyle; label: string }> = [
  { style: 'flat', label: 'Flat' },
  { style: '3d', label: 'No Flat' },
]

function isThemeStyle(v: unknown): v is ThemeStyle {
  return styles.some((s) => s.style === v)
}

/**
 * Validates a theme/style pair, falling back field by field. Used for the
 * viewer's saved choice and for the server default (default_theme).
 */
export function resolveThemePrefs(name: unknown, style: unknown, fallback = DEFAULT_THEME): ThemePrefs {
  const theme = typeof name === 'string' ? findTheme(name) : undefined
  return {
    // Canonical name, so renamed themes (aliases) are stored under the new one.
    name: theme ? theme.name : fallback.name,
    style: isThemeStyle(style) ? style : fallback.style,
  }
}

/** The viewer's saved choice, or null; storage may be unavailable (private mode, blocked). */
export function loadSavedThemePrefs(): ThemePrefs | null {
  try {
    const raw = localStorage.getItem(storageKey(STORAGE_NAME))
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<ThemePrefs>
    return resolveThemePrefs(parsed.name, parsed.style)
  } catch {
    return null
  }
}

export function saveThemePrefs(prefs: ThemePrefs): void {
  try {
    localStorage.setItem(storageKey(STORAGE_NAME), JSON.stringify(prefs))
  } catch {
    // Preference is a convenience only.
  }
}

/** camelCase token -> --c-kebab-case CSS variable. */
function cssVar(prefix: string, key: string): string {
  return `--${prefix}-${key.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}`
}

function gradient(stops: readonly string[]): string {
  return stops.length === 1 ? stops[0]! : `linear-gradient(180deg, ${stops.join(', ')})`
}

/** Hex color -> "h, s%, l%" components used by DHTMLX HSL variables. */
export function hexToHsl(hex: string): { h: number; s: number; l: number } {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255) as [number, number, number]
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2
  if (max === min) return { h: 0, s: 0, l: Math.round(l * 100) }
  const d = max - min
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
  let h: number
  switch (max) {
    case r:
      h = (g - b) / d + (g < b ? 6 : 0)
      break
    case g:
      h = (b - r) / d + 2
      break
    default:
      h = (r - g) / d + 4
  }
  return { h: Math.round(h * 60), s: Math.round(s * 100), l: Math.round(l * 100) }
}

/** RGB components of the shadow color: white on dark themes, slate on light ones. */
export function themeShadowRgb(theme: ThemeDefinition): string {
  if (theme.shadowRgb) return theme.shadowRgb
  return theme.scheme === 'dark' ? '255, 255, 255' : '15, 23, 42'
}

/** All CSS variables for a theme and style (pure, unit-tested). */
export function themeVariables(theme: ThemeDefinition, style: ThemeStyle): Record<string, string> {
  const c = theme.colors
  const vars: Record<string, string> = { '--app-font': theme.font }

  for (const [key, value] of Object.entries(c)) vars[cssVar('c', key)] = value

  // Backgrounds that the 3D style can replace with gradients.
  const surfaces: Record<keyof ThemeGradients, string> = {
    toolbar: c.toolbarBg,
    tab: c.tabBg,
    tabActive: c.tabActiveBg,
    button: c.buttonBg,
    buttonPrimary: c.primary,
    cardHeader: c.cardHeaderBg,
    panelTitle: c.panelTitleBg,
    panes: c.ink,
  }
  for (const [key, solid] of Object.entries(surfaces) as Array<[keyof ThemeGradients, string]>) {
    const stops = style === '3d' ? theme.gradients[key] : undefined
    vars[cssVar('g', key)] = stops?.length ? gradient(stops) : solid
  }
  vars['--button-hover-ring'] = theme.buttonHoverRing ?? 'transparent'
  vars['--card-header-border'] = theme.cardHeaderBorder ?? `1px solid ${c.line}`
  vars['--text-glow'] = style === '3d' && theme.glow ? theme.glow : 'none'
  const bevel = style === '3d' ? theme.bevel : undefined
  vars['--btn-shadow-3d'] = bevel
    ? `inset -1px -1px ${bevel.darkShadow}, inset 1px 1px ${bevel.light}, inset -2px -2px ${bevel.shadow}`
    : 'inset 0 1px 0 rgba(255, 255, 255, 0.16)'
  vars['--tab-accent-shadow'] =
    theme.tabAccentEdge === 'top' ? `inset 0 3px 0 ${c.tabAccent}` : `inset 0 -2px 0 ${c.tabAccent}`
  // Shadows: black is invisible on dark backgrounds, so dark themes cast a
  // light (white, or theme-tinted) shadow instead.
  const shadow = themeShadowRgb(theme)
  const dark = theme.scheme === 'dark'
  vars['--shadow-card'] = bevel
    ? `inset 1px 1px ${bevel.light}, inset -1px -1px ${bevel.darkShadow}, 2px 2px 0 rgba(0, 0, 0, 0.25)`
    : style === '3d'
      ? dark
        ? // 3D: a soft light shadow, a little stronger than the flat halo.
          `0 0 0 1px rgba(${shadow}, 0.04), 0 4px 16px rgba(${shadow}, 0.09)`
        : `0 1px 0 rgba(255, 255, 255, 0.9) inset, 0 4px 14px rgba(${shadow}, 0.18)`
      : dark
        ? // Flat: a very soft light halo, just enough to lift cards off the dark page.
          `0 0 0 1px rgba(${shadow}, 0.03), 0 2px 12px rgba(${shadow}, 0.06)`
        : 'none'
  vars['--shadow-menu'] = dark ? `0 8px 24px rgba(${shadow}, 0.1)` : `0 8px 24px rgba(${shadow}, 0.35)`

  // DHTMLX widgets (menus, tree, form, window) follow the same palette.
  const primary = hexToHsl(c.primary)
  Object.assign(vars, {
    '--dhx-font-family': theme.font,
    '--dhx-h-primary': String(primary.h),
    '--dhx-s-primary': `${primary.s}%`,
    '--dhx-l-primary': `${primary.l}%`,
    '--dhx-l-contrast-offset': '0%',
    '--dhx-color-primary': c.primary,
    '--dhx-color-primary-hover': c.primaryHover,
    '--dhx-color-primary-active': c.primaryPressed,
    '--dhx-color-focused': c.focus,
    '--dhx-background-primary': c.surface,
    '--dhx-background-secondary': c.surfaceAlt,
    '--dhx-background-additional': c.surfaceAlt,
    '--dhx-font-color-primary': c.text,
    '--dhx-font-color-secondary': c.muted,
    '--dhx-font-color-additional': c.muted,
    '--dhx-font-color-contrast': c.onPrimary,
    '--dhx-border-color': c.line,
    '--dhx-border-color-focused': c.focus,
    '--dhx-s-toolbar-background': c.toolbarBg,
    '--dhx-s-toolbar-button-background-hover': c.toolbarHoverBg,
    '--dhx-s-toolbar-button-background-active': c.selection,
    '--dhx-border-radius': '0',
  })
  return vars
}

/** Applies a theme on <html>: data attributes for CSS hooks plus all variables. */
export function applyTheme(prefs: ThemePrefs, root: HTMLElement = document.documentElement): void {
  const theme = findTheme(prefs.name) ?? THEMES[0]!
  root.dataset.appTheme = theme.name
  root.dataset.appStyle = prefs.style
  root.dataset.dhxTheme = theme.scheme
  root.style.colorScheme = theme.scheme
  for (const [name, value] of Object.entries(themeVariables(theme, prefs.style))) {
    root.style.setProperty(name, value)
  }
}

/** Theme label in a locale: its translation when it has one, else the English label. */
export function themeLabel(theme: ThemeDefinition, locale: Locale = currentLocale.value): string {
  return theme.labels?.[locale] ?? theme.label
}

/** Class marking the active theme/style (bold text and an accent bar, no icon). */
export const CURRENT_CSS = 'menu-current'

/** Toolbar dropdown to pick theme (grouped) and style; active options are highlighted. */
export function themeMenu(prefs: ThemePrefs): ToolbarElement {
  const current = (on: boolean) => (on ? { css: CURRENT_CSS } : {})
  const items: ToolbarElement[] = []
  for (const { group } of THEME_GROUPS) {
    const members = THEMES.filter((m) => m.group === group)
    if (members.length === 0) continue
    items.push({
      id: `theme-group-${group}`,
      type: 'menuItem',
      value: t(`themeGroup.${group}`),
      icon: 'mdi mdi-palette-outline',
      ...current(members.some((m) => m.name === prefs.name)),
      items: members.map((theme) => ({
        id: `theme-${theme.name}`,
        type: 'menuItem',
        value: themeLabel(theme),
        ...current(prefs.name === theme.name),
      })),
    })
  }
  items.push({ id: 'theme-sep', type: 'separator' })
  for (const s of styles) {
    items.push({
      id: `style-${s.style}`,
      type: 'menuItem',
      value: s.label,
      ...current(prefs.style === s.style),
    })
  }
  return { id: 'theme', type: 'navItem', value: t('toolbar.theme'), icon: 'mdi mdi-palette', items }
}

/** Parses a theme menu click into a preference change, or null for other items. */
export function parseThemeId(id: string | number): Partial<ThemePrefs> | null {
  const value = String(id)
  if (value.startsWith('theme-group-')) return null
  if (value.startsWith('theme-')) {
    const name = value.slice('theme-'.length)
    return findTheme(name) ? { name } : null
  }
  if (value.startsWith('style-')) {
    const style = value.slice('style-'.length)
    return isThemeStyle(style) ? { style } : null
  }
  return null
}
