import { expect, test } from '@playwright/test'

import {
  closeMenus,
  expectNoRoundedCorners,
  isolateNetwork,
  menuItem,
  ORIGIN,
  openFromMenu,
  setTheme,
  tab,
  toolbarButton,
  watchCSP,
} from './helpers'

test.beforeEach(async ({ page }) => {
  // Start every test with default preferences, but keep them across reloads.
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('e2e-cleared')) {
      localStorage.clear()
      sessionStorage.setItem('e2e-cleared', '1')
    }
  })
})

test('loads offline with no CSP violations and no external requests', async ({ page }) => {
  const external = await isolateNetwork(page)
  const csp = watchCSP(page)
  await page.goto('/')

  await expect(toolbarButton(page, 'Development')).toBeVisible()
  await expect(page.getByTestId('home')).toBeVisible()
  await expect(tab(page, 'Home')).toBeVisible()
  await expect(toolbarButton(page, 'Home')).toBeVisible()
  await expect(toolbarButton(page, 'Admin')).toBeVisible()
  await openFromMenu(page, 'Development')
  await expect(menuItem(page, 'Documentation')).toBeVisible()

  expect(external).toEqual([])
  expect(csp).toEqual([])
})

test('menus nest three levels deep and groups with URL get an "Open" entry', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')

  await openFromMenu(page, 'Development', 'Documentation')
  await expect(menuItem(page, 'Go')).toBeVisible()

  await closeMenus(page)
  await openFromMenu(page, 'Examples')
  await expect(menuItem(page, 'Open Examples')).toBeVisible()
  await menuItem(page, 'Open Examples').click()
  await expect(tab(page, 'Examples')).toBeVisible()
  await expect(page).toHaveURL(/#item-\d+$/)
})

test('tabs open, are reused without reloading, keep state, close and respect the limit', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')

  await openFromMenu(page, 'Tests', 'Page A')
  const paneA = page.locator('.pane:visible iframe')
  await expect(paneA).toHaveAttribute('src', 'https://a.e2e.test/')
  await expect(paneA).toHaveAttribute('sandbox', 'allow-scripts allow-same-origin allow-forms allow-popups')
  await page.frameLocator('.pane:visible iframe').locator('#state').fill('preserved state')

  await openFromMenu(page, 'Tests', 'Page B')
  await expect(tab(page, 'Page B')).toBeVisible()

  // Reopening A from the menu reuses the tab and keeps the iframe state.
  await openFromMenu(page, 'Tests', 'Page A')
  await expect(page.locator('.dhx_tabbar-tab')).toHaveCount(3)
  await expect(page.frameLocator('.pane:visible iframe').locator('#state')).toHaveValue('preserved state')

  // Browser back returns to the previously active tab.
  await page.goBack()
  await expect(page.getByTestId('context-title').locator('visible=true')).toHaveText('Page B')

  // Close buttons stay visible in the dark theme: white on the active tab, gray on inactive ones.
  const closeColor = (title: string) =>
    tab(page, title)
      .locator('.dhx_tabbar-tab__close')
      .evaluate((el) => getComputedStyle(el).color)
  await expect.poll(() => closeColor('Page B')).toBe('rgb(230, 237, 243)')
  await expect.poll(() => closeColor('Page A')).toBe('rgb(166, 176, 189)')

  // Home tab cannot be closed; item tabs can.
  await expect(tab(page, 'Home').locator('.dhx_tabbar-tab__close')).toHaveCount(0)
  await tab(page, 'Page B').locator('.dhx_tabbar-tab__close').click()
  await expect(tab(page, 'Page B')).toHaveCount(0)

  // max-tabs is 3 in e2e: the fourth item evicts the least recently used.
  await openFromMenu(page, 'Maps', 'OpenStreetMap — São Paulo')
  await openFromMenu(page, 'Maps', 'OpenStreetMap — Rio de Janeiro')
  await openFromMenu(page, 'Tests', 'Page B')
  await expect(page.locator('.dhx_message')).toContainText('Limit of 3 tabs')
  await expect(page.locator('.dhx_tabbar-tab')).toHaveCount(4)

  await toolbarButton(page, 'Home').click()
  await expect(page.getByTestId('home')).toBeVisible()
  await expect(page).toHaveURL(`${ORIGIN}/`)
})

test('blank items open a new browser tab', async ({ page, context }) => {
  await isolateNetwork(page)
  await page.goto('/')
  const [popup] = await Promise.all([
    context.waitForEvent('page'),
    page.getByText('Example.net (new tab)').click(),
  ])
  expect(popup.url()).toContain('example.net')
  await expect(page.locator('.dhx_tabbar-tab')).toHaveCount(1)
})

test('deep links', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/#item-999999')
  await expect(page.locator('.dhx_message')).toContainText('Item not found')
  await expect(page.getByTestId('home')).toBeVisible()

  await page.goto('/')
  await openFromMenu(page, 'Tests', 'Page A')
  const url = page.url()
  await page.goto('about:blank')
  await page.goto(url)
  await expect(tab(page, 'Page A')).toBeVisible()
})

test('security: titles are text, same-origin and top navigation are blocked', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')

  await toolbarButton(page, 'Tests').click()
  await expect(menuItem(page, '<img src=x onerror=window.__xss=1>')).toBeVisible()
  expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined()
  await closeMenus(page)

  await openFromMenu(page, 'Tests', 'Same origin')
  await expect(page.getByText('Open this item in a new tab')).toBeVisible()

  await openFromMenu(page, 'Tests', 'Window hijack')
  await page.waitForTimeout(500)
  expect(page.url().startsWith(ORIGIN)).toBe(true)
})

test('themes switch, persist and never round corners', async ({ page }) => {
  // Walks every theme/style through the animated menus: longer than the default 30s.
  test.setTimeout(120_000)
  await isolateNetwork(page)
  await page.goto('/')
  const html = page.locator('html')

  await expect(html).toHaveAttribute('data-app-theme', 'blue-black')
  await expect(page.locator('.dhx_toolbar.app-toolbar')).toHaveCSS('background-color', 'rgb(0, 0, 0)')
  await expectNoRoundedCorners(page)
  const scrollbar = await page.evaluate(() => {
    const style = getComputedStyle(document.querySelector('[data-testid=home]')!)
    return { width: style.scrollbarWidth, color: style.scrollbarColor }
  })
  expect(scrollbar).toEqual({ width: 'thin', color: 'rgb(107, 114, 128) rgb(0, 0, 0)' })

  for (const [theme, style] of [
    ['Windows XP', 'Flat'],
    ['Windows XP', 'No Flat'],
    ['Graphite', 'Flat'],
    ['Midnight', 'No Flat'],
    ['Light', 'Flat'],
    ['Light', 'No Flat'],
    ['High Contrast', 'Flat'],
    ['Dark', 'Flat'],
    ['Ocean Light', 'Flat'],
    ['Ocean Dark', 'No Flat'],
    ['Windows 98', 'No Flat'],
    ['Classic Mac OS', 'No Flat'],
    ['Matrix', 'No Flat'],
    ['Google', 'Flat'],
    ['Microsoft', 'No Flat'],
    ['Super Mario Galaxy', 'No Flat'],
    ['Brasil', 'Flat'],
    ['Brasil', 'No Flat'],
    ['Bio Manager', 'Flat'],
    ['Bio Manager', 'No Flat'],
    ['Winamp', 'Flat'],
    ['Winamp', 'No Flat'],
    ['Windows Vista', 'Flat'],
    ['Windows Vista', 'No Flat'],
    ['Windows 7', 'Flat'],
    ['Windows 7', 'No Flat'],
    ['Windows 10', 'Flat'],
    ['Windows 10', 'No Flat'],
    ['Linux KDE', 'Flat'],
    ['Linux KDE', 'No Flat'],
    ['Ubuntu', 'Flat'],
    ['Ubuntu', 'No Flat'],
    ['macOS', 'Flat'],
    ['macOS', 'No Flat'],
    ['xterm', 'Flat'],
    ['xterm', 'No Flat'],
    ['Blue and Black', 'No Flat'],
  ] as const) {
    await setTheme(page, theme, style)
    await openFromMenu(page, 'Tests', 'Page A')
    await expectNoRoundedCorners(page)
    // Toolbar icons always use the toolbar text color (never vanish on the bar).
    const colors = await toolbarButton(page, 'Maps').evaluate((btn) => ({
      text: getComputedStyle(btn).color,
      icon: getComputedStyle(btn.querySelector('.dhx_toolbar-button__icon')!).color,
    }))
    expect(colors.icon).toBe(colors.text)
    await toolbarButton(page, 'Home').click()
  }

  // Flat dark themes get a very soft light shadow too.
  await setTheme(page, 'Blue and Black', 'Flat')
  const flatShadow = await page
    .locator('[data-testid=card-grid] > section')
    .first()
    .evaluate((el) => getComputedStyle(el).boxShadow)
  expect(flatShadow).toContain('rgba(255, 255, 255, 0.06)')
  await setTheme(page, 'Blue and Black', 'No Flat')

  // Dark themes cast light shadows (the loop ends on Blue and Black, 3D).
  await expect(page.locator('html')).toHaveAttribute('data-app-theme', 'blue-black')
  const cardShadow = await page
    .locator('[data-testid=card-grid] > section')
    .first()
    .evaluate((el) => getComputedStyle(el).boxShadow)
  expect(cardShadow).toContain('rgba(255, 255, 255')
  await toolbarButton(page, 'Maps').click()
  const menuShadow = await page
    .locator('.dhx_widget.dhx_menu')
    .first()
    .evaluate((el) => getComputedStyle(el).boxShadow)
  expect(menuShadow).toContain('rgba(255, 255, 255')
  await closeMenus(page)

  await setTheme(page, 'Windows XP', 'No Flat')
  await page.reload()
  await expect(html).toHaveAttribute('data-app-theme', 'xp')
  await expect(html).toHaveAttribute('data-app-style', '3d')
  await expect(html).toHaveAttribute('data-dhx-theme', 'light')
})

test('the active theme and style are bold, without icons shifting the labels', async ({ page }) => {
  await page.goto('/')
  await setTheme(page, 'Ubuntu', 'No Flat')
  await toolbarButton(page, 'Theme').click()
  await menuItem(page, 'Systems').click()
  const weight = (text: string) =>
    menuItem(page, text)
      .locator('.dhx_menu-button__text')
      .evaluate((el) => Number(getComputedStyle(el).fontWeight))
  expect(await weight('Ubuntu')).toBeGreaterThanOrEqual(700)
  expect(await weight('macOS')).toBeLessThan(700)
  expect(await weight('Systems')).toBeGreaterThanOrEqual(700)
  // Every option label starts at the same x: no check/blank icon column.
  const left = (text: string) =>
    menuItem(page, text)
      .locator('.dhx_menu-button__text')
      .evaluate((el) => el.getBoundingClientRect().left)
  expect(await left('Ubuntu')).toBe(await left('macOS'))
  expect(await menuItem(page, 'Ubuntu').locator('.mdi-check').count()).toBe(0)
  // Hovered (selection background), the bar switches to the selection text color.
  const current = menuItem(page, 'Ubuntu')
  await current.hover()
  const hovered = await current.evaluate((el) => ({
    bar: getComputedStyle(el).boxShadow,
    text: getComputedStyle(el).color,
  }))
  expect(hovered.bar).toContain(hovered.text)
  await page.screenshot({ path: 'test-results/theme-menu-current.png' })
  await page.keyboard.press('Escape')
})

test('menus ease in, and stay still under reduced motion', async ({ page }) => {
  const openThemeMenu = async () => {
    await toolbarButton(page, 'Theme').click()
    return page
      .locator('.dhx_widget.dhx_menu[aria-label="Theme"]')
      .evaluate((el) => getComputedStyle(el).animationName)
  }
  await page.goto('/')
  expect(await openThemeMenu()).toBe('menu-enter')
  await expect(toolbarButton(page, 'Theme').locator('.dhx_toolbar-button__caret')).toHaveCSS(
    'transform',
    'matrix(0, -1, 1, 0, 0, 0)',
  )
  await page.keyboard.press('Escape')

  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.reload()
  expect(await openThemeMenu()).toBe('none')
})

test('cards move by dragging their header (no arrange mode); the order stays in this browser', async ({
  page,
  browser,
}) => {
  await isolateNetwork(page)
  await page.goto('/')
  const cardIds = (p = page) =>
    p
      .locator('[data-testid=card-grid] > section')
      .evaluateAll((els) => els.map((el) => el.getAttribute('data-testid')!.replace('card-', '')))
  await expect(page.locator('[data-testid=card-grid] > section').first()).toBeVisible()
  const before = await cardIds()
  expect(before.length).toBeGreaterThanOrEqual(3)
  const writes: string[] = []
  page.on('request', (r) => {
    if (r.method() !== 'GET' && r.url().includes('/api/')) writes.push(r.url())
  })
  await expect(page.getByTestId('arrange-start')).toHaveCount(0)

  // Drag the last card by its header onto the first one: no login, no mode to enter.
  const last = before[before.length - 1]!
  const handle = page.getByTestId(`card-handle-${last}`)
  await expect(handle).toHaveCSS('cursor', 'grab')
  await handle.dragTo(page.getByTestId(`card-${before[0]}`))
  await expect.poll(() => cardIds()).toEqual([last, ...before.slice(0, -1)])

  // Click/keyboard alternative: the move buttons show on hover/focus; focus stays on the card.
  const right = page.getByTestId(`move-right-${last}`)
  const tools = page.locator(`[data-testid=card-${last}] .card-tools`)
  await page.mouse.move(1, 1)
  await expect(tools).toHaveCSS('opacity', '0')
  await right.focus()
  await expect(tools).toHaveCSS('opacity', '1')
  await page.keyboard.press('Enter')
  const mine = [before[0]!, last, ...before.slice(1, -1)]
  await expect.poll(() => cardIds()).toEqual(mine)
  await expect(page.getByTestId(`move-right-${last}`)).toBeFocused()
  await page.getByTestId(`move-left-${last}`).click()
  await expect.poll(() => cardIds()).toEqual([last, ...before.slice(0, -1)])
  await page.getByTestId(`move-right-${last}`).click()
  await expect.poll(() => cardIds()).toEqual(mine)

  // Links stay clickable while cards are movable.
  await page.locator(`[data-testid=card-${before[0]}] .link-item`).first().click()
  await toolbarButton(page, 'Home').click()

  // Kept in this browser across reloads, and never sent to the server.
  await page.reload()
  await expect.poll(() => cardIds()).toEqual(mine)
  expect(writes).toEqual([])

  // Another browser (fresh profile) still sees the default order.
  const other = await browser.newPage()
  await other.goto('/')
  await expect.poll(() => cardIds(other)).toEqual(before)
  await other.close()
})

test('items appear only where their placement says', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')
  await expect(toolbarButton(page, 'Menu only')).toBeVisible()
  await expect(toolbarButton(page, 'Card only')).toHaveCount(0)
  await expect(page.getByTestId('home').getByText('Menu only')).toHaveCount(0)
  await expect(page.getByTestId('home').getByText('Card only')).toBeVisible()
  await page.getByTestId('home').getByText('Card link').click()
  await expect(tab(page, 'Card link')).toBeVisible()
})

test('card header icons line up with the link icons below them', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')
  await expect(page.locator('[data-testid=card-grid] > section').first()).toBeVisible()
  const offsets = await page.locator('[data-testid=card-grid] > section').evaluateAll((cards) =>
    cards.flatMap((card) => {
      const header = card.querySelector('.card-header > i')!.getBoundingClientRect()
      return [...card.querySelectorAll('.link-item > i:first-child')].map((icon) => {
        const r = icon.getBoundingClientRect()
        return { left: Math.abs(r.left - header.left), width: Math.abs(r.width - header.width) }
      })
    }),
  )
  expect(offsets.length).toBeGreaterThan(3)
  for (const o of offsets) {
    expect(o.left).toBeLessThanOrEqual(1)
    expect(o.width).toBeLessThanOrEqual(1)
  }
})

test('first visit uses the server default theme; the analyst choice wins afterwards', async ({ page }) => {
  // Port 8098 runs with the built-in default (default_theme = ocean-light).
  await page.goto('http://127.0.0.1:8098/')
  const html = page.locator('html')
  await expect(html).toHaveAttribute('data-app-theme', 'ocean-light')
  await expect(html).toHaveAttribute('data-app-style', 'flat')
  await expect(toolbarButton(page, 'Home')).toBeVisible()
  await expect(toolbarButton(page, 'Admin')).toBeVisible()
  await expect(tab(page, 'Home')).toBeVisible()

  await setTheme(page, 'Matrix', 'No Flat')
  await page.reload()
  await expect(html).toHaveAttribute('data-app-theme', 'matrix')
  await expect(html).toHaveAttribute('data-app-style', '3d')
})

test('a slow /api/config never blocks the first render', async ({ page }) => {
  await isolateNetwork(page)
  // Hold the config request far beyond the client timeout.
  await page.route('**/api/config', () => new Promise(() => {}))
  const started = Date.now()
  await page.goto('/')
  await expect(toolbarButton(page, 'Home')).toBeVisible({ timeout: 5000 })
  expect(Date.now() - started).toBeLessThan(5000)
  await expect(page.locator('html')).toHaveAttribute('data-app-theme', 'ocean-light')
})

test('the language switches to Portuguese (Brasil) and persists', async ({ page }) => {
  await isolateNetwork(page)
  await page.goto('/')
  const html = page.locator('html')
  await expect(html).toHaveAttribute('lang', 'en')
  await expect(toolbarButton(page, 'Theme')).toBeVisible()

  await toolbarButton(page, 'Language').click()
  await menuItem(page, 'Português (Brasil)').click()
  await expect(html).toHaveAttribute('lang', 'pt-BR')
  await expect(toolbarButton(page, 'Tema')).toBeVisible()
  await expect(toolbarButton(page, 'Idioma')).toBeVisible()
  // Menu data (titles) is not translated; built-in entries are.
  await openFromMenu(page, 'Examples')
  await expect(menuItem(page, 'Abrir Examples')).toBeVisible()
  await closeMenus(page)

  await page.reload()
  await expect(html).toHaveAttribute('lang', 'pt-BR')
  await expect(toolbarButton(page, 'Tema')).toBeVisible()
  await openFromMenu(page, 'Tests', 'Same origin')
  await expect(page.getByText('Abra este item em nova aba')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Recarregar' }).locator('visible=true')).toBeVisible()

  await toolbarButton(page, 'Idioma').click()
  await menuItem(page, 'English').click()
  await expect(html).toHaveAttribute('lang', 'en')
  await expect(toolbarButton(page, 'Theme')).toBeVisible()
})
