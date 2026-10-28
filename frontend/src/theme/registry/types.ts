/**
 * A theme is pure data: color tokens (hex), a font stack and optional
 * gradients used by the "3D" style. `applyTheme` turns it into CSS variables;
 * `src/theme/theme.css` only consumes those variables. To add a theme, create
 * a file in this folder and register it in `index.ts` — the contrast test
 * (WCAG AAA for text) runs on every registered theme automatically.
 */
export interface ThemeColors {
  /** Page background behind everything. */
  ink: string
  /** Cards, dropdowns, forms, inactive surfaces. */
  surface: string
  /** Slightly raised surface (hover rows, card bodies in some themes). */
  surfaceAlt: string
  line: string
  text: string
  muted: string
  link: string
  danger: string
  focus: string

  primary: string
  primaryHover: string
  primaryPressed: string
  onPrimary: string
  selection: string
  onSelection: string

  toolbarBg: string
  toolbarFg: string
  toolbarLine: string
  toolbarHoverBg: string
  toolbarHoverFg: string

  tabbarBg: string
  tabBg: string
  tabFg: string
  tabLine: string
  tabActiveBg: string
  tabActiveFg: string
  tabAccent: string
  tabCloseFg: string
  tabCloseActiveFg: string
  tabCloseHoverBg: string
  tabCloseHoverFg: string

  buttonBg: string
  buttonFg: string
  buttonLine: string
  buttonHoverBg: string
  buttonHoverFg: string

  cardHeaderBg: string
  cardHeaderFg: string
  cardBodyBg: string
  cardLinkFg: string

  panelTitleBg: string
  panelTitleFg: string
  fieldBg: string

  scrollbarTrack: string
  scrollbarThumb: string
  scrollbarThumbHover: string
}

/** Gradient stops (top to bottom) for the "3D" style; missing keys stay flat. */
export interface ThemeGradients {
  toolbar?: string[]
  tab?: string[]
  tabActive?: string[]
  button?: string[]
  buttonPrimary?: string[]
  cardHeader?: string[]
  panelTitle?: string[]
  panes?: string[]
}

export type ThemeGroup = 'dark' | 'light' | 'brand' | 'inspired' | 'system' | 'special'

export interface ThemeDefinition {
  /** Stable id stored in preferences and used in data-app-theme. */
  name: string
  /** Label shown in the UI (English). */
  label: string
  /** Translated labels by locale (e.g. `'pt-BR'`), for themes whose name is not a proper noun. */
  labels?: Readonly<Partial<Record<string, string>>>
  group: ThemeGroup
  /** Base DHTMLX theme and CSS color-scheme. */
  scheme: 'dark' | 'light'
  font: string
  colors: ThemeColors
  gradients: ThemeGradients
  /** Optional accent ring drawn on hovered buttons (e.g. XP orange). */
  buttonHoverRing?: string
  /** Edge of the active tab that carries the accent line (default bottom). */
  tabAccentEdge?: 'top' | 'bottom'
  /** Bottom border of card headers (CSS shorthand); default `1px solid <line>`. */
  cardHeaderBorder?: string
  /** Shadow color as "r, g, b"; default white on dark themes and slate on light ones. */
  shadowRgb?: string
  /** Classic raised/sunken bevels (Windows 98, Mac OS Platinum) in the 3D style. */
  bevel?: { light: string; shadow: string; darkShadow: string }
  /** Text glow (CSS text-shadow) applied to headings and links in the 3D style. */
  glow?: string
}

/** Inter is bundled with the app (src/vendor/fonts/inter, SIL OFL). */
export const INTER_FONT = "'Inter', 'Segoe UI', system-ui, sans-serif"

/** Modern sans-serif stack available offline on every OS. */
export const SYSTEM_FONT =
  "'Segoe UI Variable', 'Segoe UI', system-ui, -apple-system, Roboto, 'Helvetica Neue', Arial, sans-serif"
