import type { ThemeColors, ThemeDefinition, ThemeGradients } from '../theme/registry'

/** WCAG relative luminance of a #rrggbb color. */
export function luminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
  const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!
}

/** WCAG contrast ratio between two #rrggbb colors. */
export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi! + 0.05) / (lo! + 0.05)
}

/** Minimum ratios: WCAG AAA for text, AA for icons, 1.4.11 for UI parts. */
export const MIN_RATIO = { text: 7, icon: 4.5, ui: 3 } as const

export type PairKind = keyof typeof MIN_RATIO

export interface ColorPair {
  use: string
  kind: PairKind
  fg: string
  bg: string
}

type Key = keyof ThemeColors

/** [usage, kind, foreground token, background token, gradient that may replace the background]. */
const PAIRS: ReadonlyArray<[string, PairKind, Key, Key, (keyof ThemeGradients)?]> = [
  ['text on page', 'text', 'text', 'ink'],
  ['text on surface', 'text', 'text', 'surface'],
  ['text on raised surface', 'text', 'text', 'surfaceAlt'],
  ['muted text on surface', 'text', 'muted', 'surface'],
  ['muted text on page', 'text', 'muted', 'ink'],
  ['muted text on raised surface', 'text', 'muted', 'surfaceAlt'],
  ['link on page', 'text', 'link', 'ink'],
  ['link on surface', 'text', 'link', 'surface'],
  ['error on surface', 'text', 'danger', 'surface'],
  ['text in fields', 'text', 'text', 'fieldBg'],
  ['primary button', 'text', 'onPrimary', 'primary', 'buttonPrimary'],
  ['primary button hover', 'text', 'onPrimary', 'primaryHover'],
  ['primary button pressed', 'text', 'onPrimary', 'primaryPressed'],
  ['selection', 'text', 'onSelection', 'selection'],
  ['toolbar', 'text', 'toolbarFg', 'toolbarBg', 'toolbar'],
  ['toolbar hover', 'text', 'toolbarHoverFg', 'toolbarHoverBg'],
  ['inactive tab', 'text', 'tabFg', 'tabBg', 'tab'],
  ['active tab', 'text', 'tabActiveFg', 'tabActiveBg', 'tabActive'],
  ['button', 'text', 'buttonFg', 'buttonBg', 'button'],
  ['button hover', 'text', 'buttonHoverFg', 'buttonHoverBg'],
  ['card header', 'text', 'cardHeaderFg', 'cardHeaderBg', 'cardHeader'],
  ['card link', 'text', 'cardLinkFg', 'cardBodyBg'],
  ['panel title', 'text', 'panelTitleFg', 'panelTitleBg', 'panelTitle'],
  ['tab close (inactive)', 'icon', 'tabCloseFg', 'tabBg', 'tab'],
  ['tab close (active)', 'icon', 'tabCloseActiveFg', 'tabActiveBg', 'tabActive'],
  ['tab close hover', 'icon', 'tabCloseHoverFg', 'tabCloseHoverBg'],
  ['focus ring on page', 'ui', 'focus', 'ink'],
  ['focus ring on surface', 'ui', 'focus', 'surface'],
  ['current menu option bar on selection', 'ui', 'onSelection', 'selection'],
  ['scrollbar', 'ui', 'scrollbarThumb', 'scrollbarTrack'],
  ['active tab accent', 'ui', 'tabAccent', 'tabbarBg'],
]

/** Every foreground/background pair a theme renders, including 3D gradient stops. */
export function themePairs(theme: ThemeDefinition): ColorPair[] {
  const pairs: ColorPair[] = []
  for (const [use, kind, fg, bg, gradientKey] of PAIRS) {
    pairs.push({ use, kind, fg: theme.colors[fg], bg: theme.colors[bg] })
    const stops = gradientKey ? (theme.gradients[gradientKey] ?? []) : []
    stops.forEach((stop, i) =>
      pairs.push({ use: `${use} (3D stop ${i + 1})`, kind, fg: theme.colors[fg], bg: stop }),
    )
  }
  return pairs
}

/** Pairs below their minimum ratio (empty when the theme is compliant). */
export function contrastFailures(theme: ThemeDefinition): Array<ColorPair & { ratio: number }> {
  return themePairs(theme)
    .map((pair) => ({ ...pair, ratio: Math.round(contrastRatio(pair.fg, pair.bg) * 100) / 100 }))
    .filter((pair) => pair.ratio < MIN_RATIO[pair.kind])
}
