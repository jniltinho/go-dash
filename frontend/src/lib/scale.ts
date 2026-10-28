/**
 * UI scale shared by CSS and JS. CSS defines --ui-scale with media queries
 * (src/theme/theme.css); widgets configured in pixels from JS (DHTMLX Tabbar,
 * Layout columns) read it here and are refreshed when a breakpoint is crossed.
 */
export const SCALE_BREAKPOINTS = ['(min-width: 2300px)', '(min-width: 3400px)'] as const

export function uiScale(): number {
  const value = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--ui-scale'))
  return Number.isFinite(value) && value > 0 ? value : 1
}

/** Calls listener whenever the UI scale may have changed; returns a cleanup function. */
export function onScaleChange(listener: (scale: number) => void): () => void {
  const queries = SCALE_BREAKPOINTS.map((q) => window.matchMedia(q))
  const handler = () => listener(uiScale())
  queries.forEach((q) => q.addEventListener('change', handler))
  return () => queries.forEach((q) => q.removeEventListener('change', handler))
}
