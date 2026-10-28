export type Target = 'iframe' | 'blank'

/** Where an item is shown: header toolbar menu, home tab card, or both. */
export type Placement = 'menu' | 'card' | 'both'

export interface MenuItem {
  id: number
  parent_id: number | null
  title: string
  url: string | null
  target: Target
  placement: Placement
  icon: string | null
  description: string | null
  position: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface MenuNode extends MenuItem {
  items: MenuNode[]
}

export interface MenuInput {
  parent_id: number | null
  title: string
  url: string | null
  target: Target
  placement: Placement
  icon: string | null
  description: string | null
  position: number
  enabled: boolean
}

export interface Move {
  id: number
  parent_id: number | null
  position: number
}

export interface SessionStatus {
  admin_enabled: boolean
  authenticated: boolean
  csrf_token: string | null
}

export interface PublicConfig {
  title: string
  max_tabs: number
  iframe_sandbox: string
  public_url: string
  /** Theme for visitors without a saved choice (server config default_theme). */
  default_theme: string
  default_theme_style: string
  build: { version: string; commit: string; build_date: string }
}

export type ImportMode = 'append' | 'replace'

/** A dashboard as listed in the admin. `slug` is null for the default one (served at "/"). */
export interface Dashboard {
  id: number
  slug: string | null
  /** Empty on the default dashboard when the configured title applies. */
  title: string
  is_default: boolean
  protected: boolean
  item_count: number
  created_at: string
  updated_at: string
}

export interface DashboardInput {
  slug: string | null
  title: string
}

/** Public state of one dashboard (no menu content). */
export interface DashboardStatus {
  slug: string | null
  is_default: boolean
  title: string
  protected: boolean
  unlocked: boolean
}

/** Dashboard reference in admin URLs: a numeric id or "default". */
export type DashboardRef = number | 'default'
