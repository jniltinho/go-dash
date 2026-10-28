import { markRaw, onBeforeUnmount } from 'vue'

interface Destructible {
  destructor(): void
}

/**
 * Registers DHTMLX widgets created by a component and destroys them, in
 * reverse creation order, when the component unmounts. Widgets are marked raw
 * so Vue never makes them reactive.
 */
export function useDhx() {
  const widgets: Destructible[] = []

  function own<T extends Destructible>(widget: T): T {
    widgets.push(widget)
    return markRaw(widget)
  }

  /** Destroys a widget before unmount and stops tracking it. */
  function release(widget: Destructible): void {
    const index = widgets.indexOf(widget)
    if (index !== -1) widgets.splice(index, 1)
    widget.destructor()
  }

  onBeforeUnmount(() => {
    while (widgets.length > 0) {
      try {
        widgets.pop()!.destructor()
      } catch {
        // A parent destructor may already have destroyed an attached child.
      }
    }
  })

  return { own, release }
}

/**
 * Container for widgets that will be attached to a Layout cell or Window.
 * DHTMLX accepts null here, but its typings require an element.
 */
export const DETACHED = null as unknown as HTMLElement
