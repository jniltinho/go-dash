import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vite-plus/test'

import type { MenuNode } from '../api/types'
import { THEMES } from '../theme/registry'
import {
  applyCardOrder,
  cardOrderKey,
  LEGACY_CARD_ORDER_KEY,
  migrateLegacyCardOrder,
  parseCardOrder,
} from './cardOrder'
import { ApiError } from '../api/client'
import {
  classifyMenuError,
  dashboardLabel,
  dashboardPath,
  normalizeSlug,
  RESERVED_SLUGS,
  SLUG_PATTERN,
} from './dashboard'
import { contrastFailures, themePairs } from './contrast'
import { apiUrl, basePathFrom, storageKey } from './basePath'
import { translateError } from './messages'
import { MODAL_TOP_RATIO, modalPosition } from './modal'
import {
  buildToolbar,
  cardTree,
  duplicateInput,
  MAX_TITLE_LENGTH,
  placeAfterMoves,
  menuTree,
  moveIndex,
  indexTree,
  isSameOrigin,
  parseHash,
  parseToolbarId,
  type ToolbarElement,
} from './menu'
import { iconClass, iconName, searchIcons, SUGGESTED_ICONS } from './icons'
import { TabTracker } from './tabs'
import {
  CURRENT_CSS,
  DEFAULT_THEME,
  hexToHsl,
  parseThemeId,
  resolveThemePrefs,
  themeMenu,
  themeVariables,
} from './theme'

function node(id: number, title: string, extra: Partial<MenuNode> = {}): MenuNode {
  return {
    id,
    parent_id: null,
    title,
    url: null,
    target: 'iframe',
    icon: null,
    description: null,
    placement: 'both',
    position: 0,
    enabled: true,
    created_at: '',
    updated_at: '',
    items: [],
    ...extra,
  }
}

describe('buildToolbar', () => {
  it('maps three levels and adds fixed actions', () => {
    const pg = node(3, 'pgAdmin', { url: 'https://pg.example.com' })
    const banks = node(2, 'Bancos', { items: [pg] })
    const infra = node(1, 'Infra', { items: [banks], icon: 'mdi mdi-server' })

    const items = buildToolbar('Panel', [infra])
    expect(items[0]).toMatchObject({ type: 'title', value: 'Panel' })
    const root = items.find((i) => i.id === 'item-1')!
    expect(root.type).toBe('navItem')
    expect(root.icon).toBe('mdi mdi-server')
    expect(root.items![0]!.id).toBe('item-2')
    expect(root.items![0]!.items![0]).toMatchObject({ id: 'item-3', type: 'menuItem', value: 'pgAdmin' })
    expect(items.slice(-2).map((i) => i.id)).toEqual(['action-home', 'action-admin'])
  })

  it('adds an "Open" entry for groups with URL', () => {
    const grafana = node(1, 'Grafana', {
      url: 'https://g.example.com',
      items: [node(2, 'Dashboards'), node(3, 'Alerts')],
    })
    const root = buildToolbar('P', [grafana]).find((i) => i.id === 'item-1')!
    expect(root.items!.map((i) => i.id)).toEqual(['open-1', 'sep-1', 'item-2', 'item-3'])
    expect(root.items![0]!.value).toBe('Open Grafana')
  })

  it('keeps titles as plain values', () => {
    const evil = node(1, '<img src=x onerror=alert(1)>')
    const root = buildToolbar('P', [evil]).find((i) => i.id === 'item-1')!
    expect(root.value).toBe('<img src=x onerror=alert(1)>')
    expect(root).not.toHaveProperty('html')
  })
})

describe('ids and hashes', () => {
  it('parses toolbar ids', () => {
    expect(parseToolbarId('item-12')).toEqual({ kind: 'item', id: 12 })
    expect(parseToolbarId('open-3')).toEqual({ kind: 'open', id: 3 })
    expect(parseToolbarId('action-home')).toEqual({ kind: 'home' })
    expect(parseToolbarId('action-lock')).toEqual({ kind: 'lock' })
    expect(parseToolbarId('title')).toEqual({ kind: 'none' })
  })

  it('parses hashes', () => {
    expect(parseHash('#item-7')).toBe(7)
    expect(parseHash('#foo')).toBeNull()
    expect(parseHash('')).toBeNull()
  })

  it('indexes nested nodes', () => {
    const tree = [node(1, 'a', { items: [node(2, 'b', { items: [node(3, 'c')] })] })]
    expect([...indexTree(tree).keys()]).toEqual([1, 2, 3])
  })

  it('detects same origin', () => {
    expect(isSameOrigin('http://localhost:8080/admin', 'http://localhost:8080')).toBe(true)
    expect(isSameOrigin('https://grafana.example.com', 'http://localhost:8080')).toBe(false)
    expect(isSameOrigin('not a url', 'http://localhost:8080')).toBe(false)
  })
})

describe('TabTracker', () => {
  it('reuses existing tabs and evicts the least recently used', () => {
    const tabs = new TabTracker(2)
    expect(tabs.openItem(1)).toEqual({ created: true, evicted: null })
    expect(tabs.openItem(2)).toEqual({ created: true, evicted: null })
    expect(tabs.openItem(1)).toEqual({ created: false, evicted: null })
    // 2 is now the least recently used.
    expect(tabs.openItem(3)).toEqual({ created: true, evicted: 2 })
    expect(tabs.open).toEqual([1, 3])
  })

  it('closes tabs', () => {
    const tabs = new TabTracker(5)
    tabs.openItem(1)
    tabs.openItem(2)
    tabs.close(1)
    expect(tabs.open).toEqual([2])
    expect(tabs.has(1)).toBe(false)
  })
})

describe('theme contrast (WCAG AAA for text)', () => {
  it.each(THEMES.map((t) => [t.label, t] as const))('%s has no failing pair', (_label, theme) => {
    expect(contrastFailures(theme)).toEqual([])
  })

  it('checks 3D gradient stops too', () => {
    const xp = THEMES.find((t) => t.name === 'xp')!
    expect(themePairs(xp).some((p) => p.use.startsWith('toolbar (3D stop'))).toBe(true)
  })
})

describe('theme', () => {
  it('registers unique names and a default', () => {
    const names = THEMES.map((t) => t.name)
    expect(new Set(names).size).toBe(names.length)
    expect(names[0]).toBe('blue-black')
    expect(names.length).toBeGreaterThanOrEqual(25)
    expect(names).toEqual(
      expect.arrayContaining([
        'ocean-light',
        'ocean-dark',
        'win98',
        'mac-classic',
        'matrix',
        'dark',
        'light',
        'google',
        'microsoft',
        'super-mario-galaxy',
        'brasil',
        'bio-manager',
        'winamp',
        'vista',
        'win7',
        'win10',
        'kde',
        'ubuntu',
        'macos',
        'xterm',
      ]),
    )
  })

  it('builds a grouped menu with the active options highlighted', () => {
    const menu = themeMenu({ name: 'xp', style: '3d' })
    const flat = (items: ToolbarElement[]): ToolbarElement[] =>
      items.flatMap((i) => [i, ...flat(i.items ?? [])])
    const all = flat(menu.items!)
    expect(all.filter((i) => i.css === CURRENT_CSS).map((i) => i.id)).toEqual([
      'theme-group-system',
      'theme-xp',
      'style-3d',
    ])
    // No check/blank icons: option labels stay aligned.
    expect(all.filter((i) => !i.id.startsWith('theme-group-')).every((i) => i.icon === undefined)).toBe(true)
    expect(menu.items!.filter((i) => i.id.startsWith('theme-group-')).map((i) => i.value)).toEqual([
      'Dark',
      'Light',
      'Brand',
      'Inspired',
      'Systems',
      'Special',
    ])
  })

  it('parses theme menu ids', () => {
    expect(parseThemeId('theme-xp')).toEqual({ name: 'xp' })
    expect(parseThemeId('theme-graphite')).toEqual({ name: 'graphite' })
    expect(parseThemeId('style-flat')).toEqual({ style: 'flat' })
    expect(parseThemeId('theme-group-dark')).toBeNull()
    expect(parseThemeId('theme-unknown')).toBeNull()
    expect(parseThemeId('item-1')).toBeNull()
  })

  it('adds bevels (retro) and glow (Matrix) only in 3D', () => {
    const win98 = THEMES.find((t) => t.name === 'win98')!
    expect(themeVariables(win98, '3d')['--btn-shadow-3d']).toContain('inset -1px -1px #000000')
    expect(themeVariables(win98, 'flat')['--shadow-card']).toBe('none')
    const matrix = THEMES.find((t) => t.name === 'matrix')!
    expect(themeVariables(matrix, '3d')['--text-glow']).toContain('rgba(0, 255, 65')
    expect(themeVariables(matrix, 'flat')['--text-glow']).toBe('none')
  })

  it('casts white shadows on dark themes and dark shadows on light ones', () => {
    const byName = (n: string) => THEMES.find((t) => t.name === n)!
    const dark3d = themeVariables(byName('dark'), '3d')
    expect(dark3d['--shadow-card']).toBe(
      '0 0 0 1px rgba(255, 255, 255, 0.04), 0 4px 16px rgba(255, 255, 255, 0.09)',
    )
    expect(dark3d['--shadow-menu']).toBe('0 8px 24px rgba(255, 255, 255, 0.1)')
    expect(dark3d['--shadow-menu']).toContain('rgba(255, 255, 255')
    // Flat: a much softer light shadow on dark themes, none on light themes.
    expect(themeVariables(byName('dark'), 'flat')['--shadow-card']).toBe(
      '0 0 0 1px rgba(255, 255, 255, 0.03), 0 2px 12px rgba(255, 255, 255, 0.06)',
    )
    expect(themeVariables(byName('light'), 'flat')['--shadow-card']).toBe('none')
    expect(themeVariables(byName('dark'), 'flat')['--shadow-menu']).toContain('rgba(255, 255, 255')
    expect(themeVariables(byName('light'), '3d')['--shadow-menu']).toContain('rgba(15, 23, 42')
    expect(themeVariables(byName('matrix'), '3d')['--shadow-card']).toContain('rgba(0, 255, 65')
    // Bevel themes (retro) keep their classic dark bevel shadow.
    for (const theme of THEMES.filter((t) => t.scheme === 'dark' && !t.shadowRgb && !t.bevel)) {
      expect(themeVariables(theme, '3d')['--shadow-card']).not.toMatch(/rgba\(0, 0, 0/)
    }
  })

  it('lets a theme set the card header border', () => {
    const ad = THEMES.find((t) => t.name === 'bio-manager')!
    expect(themeVariables(ad, 'flat')['--card-header-border']).toBe('2px solid #f58322')
    const blue = THEMES.find((t) => t.name === 'blue-black')!
    expect(themeVariables(blue, 'flat')['--card-header-border']).toBe('1px solid #262c36')
  })

  it('produces CSS variables, with gradients only in 3D', () => {
    const xp = THEMES.find((t) => t.name === 'xp')!
    const flatVars = themeVariables(xp, 'flat')
    const vars3d = themeVariables(xp, '3d')
    expect(flatVars['--c-toolbar-bg']).toBe('#0046be')
    expect(flatVars['--g-toolbar']).toBe('#0046be')
    expect(vars3d['--g-toolbar']).toMatch(/^linear-gradient\(180deg, #0a4fb8/)
    expect(flatVars['--dhx-border-radius']).toBe('0')
    expect(flatVars['--app-font']).toContain('Tahoma')
  })

  it('converts hex to HSL', () => {
    expect(hexToHsl('#1e40af')).toEqual({ h: 226, s: 71, l: 40 })
    expect(hexToHsl('#000000')).toEqual({ h: 0, s: 0, l: 0 })
  })

  it('inserts extra toolbar items before the fixed actions', () => {
    const items = buildToolbar('P', [], [themeMenu({ name: 'blue-black', style: 'flat' })])
    expect(items.slice(-3).map((i) => i.id)).toEqual(['theme', 'action-home', 'action-admin'])
  })
})

describe('translateError', () => {
  it('translates known API messages', () => {
    expect(translateError('URL must use http or https')).toBe('The URL must start with http:// or https://.')
    expect(translateError('item has 2 children; use force to delete them too')).toBe(
      'The item has 2 subitems.',
    )
    expect(translateError('moving item 3 would create a cycle')).toContain('cannot be placed inside itself')
  })

  it('falls back to the original message', () => {
    expect(translateError('boom')).toBe('The operation could not be completed (boom).')
  })
})

describe('icons', () => {
  const names = [
    'home',
    'home-outline',
    'house',
    'chart-line',
    'chart-bar',
    'database',
    'bank',
    'account',
    'pie',
  ]

  it('finds icons by English name, best matches first', () => {
    expect(searchIcons(names, 'home').names).toEqual(['home', 'home-outline'])
    expect(searchIcons(names, 'chart bar').names).toEqual(['chart-bar'])
  })

  it('accepts Portuguese terms without accents', () => {
    expect(searchIcons(names, 'Gráfico').names).toEqual(['chart-bar', 'chart-line'])
    expect(searchIcons(names, 'casa').names).toEqual(['home', 'house', 'home-outline'])
    expect(searchIcons(names, 'banco').names).toEqual(['bank', 'database'])
    expect(searchIcons(names, 'gráfico pizza').names).toEqual([])
    expect(searchIcons(['chart-pie', 'chart-bar'], 'gráfico pizza').names).toEqual(['chart-pie'])
  })

  it('accepts English synonyms that are not part of icon names', () => {
    expect(searchIcons(names, 'house').names).toEqual(['home', 'house', 'home-outline'])
    expect(searchIcons(names, 'user').names).toEqual(['account'])
    expect(searchIcons(names, 'graph').names).toEqual(['chart-bar', 'chart-line'])
  })

  it('limits results and reports the total', () => {
    expect(searchIcons(names, 'h', 2)).toEqual({ names: ['home', 'house'], total: 5 })
    expect(searchIcons(names, '   ')).toEqual({ names: [], total: 0 })
    // Inherited object keys are not synonyms.
    expect(searchIcons(names, 'constructor')).toEqual({ names: [], total: 0 })
  })

  it('converts between names and classes', () => {
    expect(iconClass('home')).toBe('mdi mdi-home')
    expect(iconName('mdi mdi-chart-line')).toBe('chart-line')
    expect(iconName('fa fa-home')).toBeNull()
    expect(iconName(null)).toBeNull()
  })

  it('suggests only real icon names', async () => {
    const { default: all } = await import('virtual:mdi-icons')
    expect(all.length).toBeGreaterThan(7000)
    expect(SUGGESTED_ICONS.filter((n) => !all.includes(n))).toEqual([])
  })
})

describe('duplicateInput', () => {
  it('copies the item settings under the same parent, right after it', () => {
    const original = node(7, 'Grafana', {
      parent_id: 3,
      url: 'https://grafana.example.com',
      target: 'blank',
      placement: 'card',
      icon: 'mdi mdi-chart-line',
      description: 'Metrics',
      position: 4,
      enabled: false,
      items: [node(8, 'Child')],
    })
    expect(duplicateInput(original)).toEqual({
      parent_id: 3,
      title: 'Grafana (copy)',
      url: 'https://grafana.example.com',
      target: 'blank',
      placement: 'card',
      icon: 'mdi mdi-chart-line',
      description: 'Metrics',
      position: 5,
      enabled: false,
    })
  })

  it('places the saved copy right after the original', () => {
    const siblings = [node(1, 'A'), node(2, 'B'), node(9, 'A (copy)'), node(3, 'C')]
    expect(placeAfterMoves(siblings, 1, 9, 5)).toEqual([
      { id: 1, parent_id: 5, position: 0 },
      { id: 9, parent_id: 5, position: 1 },
      { id: 2, parent_id: 5, position: 2 },
      { id: 3, parent_id: 5, position: 3 },
    ])
    expect(placeAfterMoves([node(1, 'A')], 7, 9, null).map((m) => m.id)).toEqual([1, 9])
  })

  it('keeps the copy title within the API limit', () => {
    const title = duplicateInput(node(1, 'é'.repeat(MAX_TITLE_LENGTH))).title
    expect([...title].length).toBe(MAX_TITLE_LENGTH)
    expect(title.endsWith(' (copy)')).toBe(true)
  })
})

describe('placement and card order', () => {
  const tree = [
    node(1, 'Both', {
      items: [node(11, 'Link menu', { placement: 'menu' }), node(12, 'Link card', { placement: 'card' })],
    }),
    node(2, 'Menu only', { placement: 'menu' }),
    node(3, 'Card only', { placement: 'card' }),
    node(4, 'Another card'),
  ]

  it('splits items between toolbar and cards, recursively', () => {
    expect(menuTree(tree).map((n) => n.id)).toEqual([1, 2, 4])
    expect(menuTree(tree)[0]!.items.map((n) => n.id)).toEqual([11])
    expect(cardTree(tree).map((n) => n.id)).toEqual([1, 3, 4])
    expect(cardTree(tree)[0]!.items.map((n) => n.id)).toEqual([12])
  })

  it('keeps card-only items out of the toolbar', () => {
    const ids = buildToolbar('P', tree).map((i) => i.id)
    expect(ids).toContain('item-2')
    expect(ids).not.toContain('item-3')
  })

  it('moves list elements', () => {
    expect(moveIndex(['a', 'b', 'c'], 2, 0)).toEqual(['c', 'a', 'b'])
    expect(moveIndex(['a', 'b', 'c'], 0, 5)).toEqual(['b', 'c', 'a'])
  })
})

describe('per-browser card order', () => {
  const cards = [node(1, 'A'), node(2, 'B'), node(3, 'C')]
  const ids = (list: MenuNode[]) => list.map((c) => c.id)

  it('uses the default order without a saved one', () => {
    expect(ids(applyCardOrder(cards, null))).toEqual([1, 2, 3])
    expect(ids(applyCardOrder(cards, []))).toEqual([1, 2, 3])
  })

  it('applies the saved order, drops removed cards and appends new ones', () => {
    expect(ids(applyCardOrder(cards, [3, 1, 2]))).toEqual([3, 1, 2])
    // 9 was removed by an admin; 2 is new for this browser; duplicates are ignored.
    expect(ids(applyCardOrder(cards, [3, 9, 1, 3]))).toEqual([3, 1, 2])
  })

  it('ignores malformed stored values', () => {
    expect(parseCardOrder('[3,1,2]')).toEqual([3, 1, 2])
    for (const raw of [null, '', '[]', '{}', '"x"', '[1,"2"]', '[0]', '[1.5]', 'not json']) {
      expect(parseCardOrder(raw)).toBeNull()
    }
  })
})

describe('default theme', () => {
  it('defaults to Ocean Light, flat', () => {
    expect(DEFAULT_THEME).toEqual({ name: 'ocean-light', style: 'flat' })
  })

  it('validates the server default field by field', () => {
    expect(resolveThemePrefs('matrix', '3d')).toEqual({ name: 'matrix', style: '3d' })
    expect(resolveThemePrefs('unknown', '3d')).toEqual({ name: 'ocean-light', style: '3d' })
    expect(resolveThemePrefs('xp', 'glossy')).toEqual({ name: 'xp', style: 'flat' })
    expect(resolveThemePrefs(undefined, null)).toEqual(DEFAULT_THEME)
    // Renamed themes keep working and are stored under the new name.
    expect(resolveThemePrefs('ad-manager', '3d')).toEqual({ name: 'bio-manager', style: '3d' })
    expect(resolveThemePrefs('super-mario-world', 'flat')).toEqual({
      name: 'super-mario-galaxy',
      style: 'flat',
    })
  })
})

describe('dashboards', () => {
  it('maps slugs to browser paths', () => {
    expect(dashboardPath(null)).toBe('/')
    expect(dashboardPath('default')).toBe('/')
    expect(dashboardPath('acme')).toBe('/dash/acme')
  })

  it('mirrors the server slug rules', () => {
    expect(normalizeSlug('  Over-Mind ')).toBe('over-mind')
    for (const ok of ['a', 'acme', 'globex-2', 'x'.repeat(40)]) expect(SLUG_PATTERN.test(ok)).toBe(true)
    for (const bad of ['', '-x', 'x-', 'over mind', 'Ab', 'x'.repeat(41)])
      expect(SLUG_PATTERN.test(bad)).toBe(false)
    expect(RESERVED_SLUGS.has('default')).toBe(true)
    expect(RESERVED_SLUGS.has('admin')).toBe(true)
  })

  it('labels dashboards with their path, falling back to the configured title', () => {
    expect(dashboardLabel({ slug: null, title: '' }, 'Go Dash')).toBe('Go Dash (/)')
    expect(dashboardLabel({ slug: 'initech', title: 'Initech' }, 'X')).toBe('Initech (/dash/initech)')
  })

  it('classifies menu errors', () => {
    expect(classifyMenuError(new ApiError(401, 'dashboard locked'))).toBe('locked')
    expect(classifyMenuError(new ApiError(404, 'dashboard not found'))).toBe('not-found')
    expect(classifyMenuError(new ApiError(503, 'unavailable'))).toBe('transient')
    expect(classifyMenuError(new TypeError('offline'))).toBe('transient')
  })
})

describe('card order per dashboard', () => {
  function memoryStorage(initial: Record<string, string>) {
    const data = new Map(Object.entries(initial))
    return {
      data,
      getItem: (k: string) => data.get(k) ?? null,
      setItem: (k: string, v: string) => void data.set(k, v),
      removeItem: (k: string) => void data.delete(k),
    }
  }

  it('uses one key per dashboard', () => {
    expect(cardOrderKey('default')).toBe('dashboard.cardOrder.default')
    expect(cardOrderKey('acme')).not.toBe(cardOrderKey('initech'))
  })

  it('moves the legacy order to the default dashboard once', () => {
    const storage = memoryStorage({ [LEGACY_CARD_ORDER_KEY]: '[3,1]' })
    migrateLegacyCardOrder(storage)
    expect(storage.data.get(cardOrderKey('default'))).toBe('[3,1]')
    expect(storage.data.has(LEGACY_CARD_ORDER_KEY)).toBe(false)
  })

  it('keeps an order already saved under the new key', () => {
    const storage = memoryStorage({ [LEGACY_CARD_ORDER_KEY]: '[3,1]', [cardOrderKey('default')]: '[2]' })
    migrateLegacyCardOrder(storage)
    expect(storage.data.get(cardOrderKey('default'))).toBe('[2]')
    expect(storage.data.has(LEGACY_CARD_ORDER_KEY)).toBe(false)
  })
})

describe('modal position', () => {
  it('opens 10% below the top, horizontally centered', () => {
    expect(MODAL_TOP_RATIO).toBe(0.1)
    expect(modalPosition(560, 520, { width: 1920, height: 1080 })).toEqual({ left: 680, top: 108 })
  })

  it('never goes off-screen on small viewports', () => {
    expect(modalPosition(560, 520, { width: 400, height: 560 })).toEqual({ left: 0, top: 40 })
    expect(modalPosition(560, 520, { width: 400, height: 500 })).toEqual({ left: 0, top: 0 })
  })
})

describe('base path', () => {
  it('comes from <base href>, and is "/" without one (never the current route)', () => {
    expect(basePathFrom('/panel/', 'https://host/panel/admin')).toBe('/panel/')
    expect(basePathFrom('/ops/dash', 'https://host/ops/dash/x')).toBe('/ops/dash/')
    expect(basePathFrom(null, 'http://localhost:9000/admin/login')).toBe('/')
    expect(basePathFrom('/', 'https://host/dash/acme')).toBe('/')
  })

  it('prefixes API calls', () => {
    expect(apiUrl('/api/menu', '/')).toBe('/api/menu')
    expect(apiUrl('/api/menu', '/panel/')).toBe('/panel/api/menu')
  })

  it('namespaces storage keys only under a prefix', () => {
    expect(storageKey('dashboard.theme', '/')).toBe('dashboard.theme')
    expect(storageKey('dashboard.theme', '/panel/')).toBe('/panel/dashboard.theme')
    expect(cardOrderKey('default', '/panel/')).toBe('/panel/dashboard.cardOrder.default')
  })

  it('leaves the legacy card order to the instance at "/"', () => {
    const data = new Map([[LEGACY_CARD_ORDER_KEY, '[3,1]']])
    const storage = {
      getItem: (k: string) => data.get(k) ?? null,
      setItem: (k: string, v: string) => void data.set(k, v),
      removeItem: (k: string) => void data.delete(k),
    }
    migrateLegacyCardOrder(storage, '/panel/')
    expect([...data.keys()]).toEqual([LEGACY_CARD_ORDER_KEY])
  })

  it('has no root-absolute or bare hash links in the UI', () => {
    const src = fileURLToPath(new URL('..', import.meta.url))
    const vueFiles = (dir: string): string[] =>
      readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
        e.isDirectory() ? vueFiles(join(dir, e.name)) : e.name.endsWith('.vue') ? [join(dir, e.name)] : [],
      )
    const offenders = vueFiles(src).filter((f) => /\shref="[/#]/.test(readFileSync(f, 'utf8')))
    expect(offenders).toEqual([])
  })
})
