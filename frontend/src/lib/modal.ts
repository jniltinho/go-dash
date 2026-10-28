/** Every modal opens this fraction of the viewport height below the top (see theme/modal.css). */
export const MODAL_TOP_RATIO = 0.1

/**
 * Position for a DHTMLX Window of the given size: horizontally centered and
 * MODAL_TOP_RATIO below the top, never off-screen.
 */
export function modalPosition(
  width: number,
  height: number,
  viewport: { width: number; height: number },
): { left: number; top: number } {
  const left = Math.max(0, Math.round((viewport.width - width) / 2))
  const top = Math.max(0, Math.min(Math.round(viewport.height * MODAL_TOP_RATIO), viewport.height - height))
  return { left, top }
}
