/** Result of opening an item tab. */
export interface OpenResult {
  /** true when a new tab must be created, false when an existing one is reused. */
  created: boolean
  /** Item evicted to respect the tab limit (least recently used). */
  evicted: number | null
}

/**
 * Tracks open item tabs in least-recently-used order. The home tab is not
 * tracked and never evicted.
 */
export class TabTracker {
  private order: number[] = []
  readonly limit: number

  constructor(limit: number) {
    this.limit = Math.max(1, limit)
  }

  /** IDs of open items, least recently used first. */
  get open(): readonly number[] {
    return this.order
  }

  has(id: number): boolean {
    return this.order.includes(id)
  }

  /** Marks id as used, opening it if needed and evicting the LRU tab over the limit. */
  openItem(id: number): OpenResult {
    if (this.has(id)) {
      this.touch(id)
      return { created: false, evicted: null }
    }
    let evicted: number | null = null
    if (this.order.length >= this.limit) {
      evicted = this.order.shift() ?? null
    }
    this.order.push(id)
    return { created: true, evicted }
  }

  /** Moves id to the most recently used position. */
  touch(id: number): void {
    const index = this.order.indexOf(id)
    if (index === -1) return
    this.order.splice(index, 1)
    this.order.push(id)
  }

  close(id: number): void {
    this.order = this.order.filter((value) => value !== id)
  }
}
