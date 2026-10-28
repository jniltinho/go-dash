import { expect, test, type Page } from '@playwright/test'

/** Output folder: docs/screenshots (relative to frontend/). */
const OUT = '../docs/screenshots'

/** Every theme in the flat style, plus the 3D style where it changes the look most. */
const SHOTS: Array<[name: string, style: 'flat' | '3d']> = [
  ['ocean-light', 'flat'],
  ['ocean-dark', 'flat'],
  ['bio-manager', 'flat'],
  ['bio-manager', '3d'],
  ['blue-black', 'flat'],
  ['dark', 'flat'],
  ['graphite', 'flat'],
  ['midnight', 'flat'],
  ['light', 'flat'],
  ['win10', 'flat'],
  ['win10', '3d'],
  ['kde', 'flat'],
  ['kde', '3d'],
  ['ubuntu', 'flat'],
  ['ubuntu', '3d'],
  ['macos', 'flat'],
  ['macos', '3d'],
  ['xterm', 'flat'],
  ['xterm', '3d'],
  ['win7', 'flat'],
  ['win7', '3d'],
  ['vista', 'flat'],
  ['vista', '3d'],
  ['xp', 'flat'],
  ['win98', 'flat'],
  ['mac-classic', 'flat'],
  ['winamp', 'flat'],
  ['winamp', '3d'],
  ['matrix', 'flat'],
  ['high-contrast', 'flat'],
  ['google', 'flat'],
  ['microsoft', 'flat'],
  ['super-mario-galaxy', 'flat'],
  ['super-mario-galaxy', '3d'],
  ['brasil', 'flat'],
  ['brasil', '3d'],
  ['ocean-dark', '3d'],
  ['xp', '3d'],
  ['win98', '3d'],
  ['mac-classic', '3d'],
  ['matrix', '3d'],
]

async function prepare(page: Page, name: string, style: string): Promise<void> {
  await page.addInitScript(
    (value) => localStorage.setItem('dashboard.theme', value),
    JSON.stringify({ name, style }),
  )
  // Never reach external sites: embedded pages get a neutral placeholder.
  await page.route(/^https?:\/\/(?!127\.0\.0\.1)/, (route) =>
    route.fulfill({ contentType: 'text/html', body: '<!doctype html><title>demo</title>' }),
  )
}

async function homeWithTabsAndMenu(page: Page): Promise<void> {
  await page.goto('/')
  const card = (text: string) => page.locator('.link-item').filter({ hasText: text }).first()
  await card('OpenStreetMap — São Paulo').click()
  await page.locator('.dhx_tabbar-tab-button').filter({ hasText: 'Home' }).click()
  await card('Example.org').click()
  await page.locator('.dhx_tabbar-tab-button').filter({ hasText: 'Home' }).click()
  await page
    .locator('.app-toolbar button')
    .filter({ hasText: /^\s*Maps\s*$/ })
    .click()
  await page.locator('.dhx_menu-button').first().hover()
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(250)
}

for (const [name, style] of SHOTS) {
  test(`theme ${name} ${style}`, async ({ page }) => {
    await prepare(page, name, style)
    await homeWithTabsAndMenu(page)
    await page.screenshot({ path: `${OUT}/theme-${name}-${style}.png`, animations: 'disabled' })
  })
}

test('admin with icon picker (Oceano Claro)', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 })
  await prepare(page, 'ocean-light', 'flat')
  await page.goto('/admin/login')
  await page.getByTestId('login-password').fill('screenshots-only')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await page
    .locator('.dhx_tree-list-item__text')
    .filter({ hasText: /^Gitea/ })
    .click()
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: `${OUT}/admin-ocean-light.png`, animations: 'disabled' })

  await page.locator('.field-action-button button').filter({ hasText: 'Choose' }).click()
  await page.getByTestId('icon-search').fill('server')
  await page.locator('.icon-cell[data-icon="server"]').click()
  await page.waitForTimeout(200)
  await page.screenshot({ path: `${OUT}/icon-picker-ocean-light.png`, animations: 'disabled' })
})

test('arranging cards (Oceano Claro)', async ({ page }) => {
  await prepare(page, 'ocean-light', 'flat')
  await page.goto('/')
  // Hovering a card shows the grip on its header (the header is the drag handle).
  await page.locator('[data-testid=card-grid] > section .card-handle').first().hover()
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: `${OUT}/arrange-cards-ocean-light.png`, animations: 'disabled' })
})

test('dashboards panel and unlock screen (Oceano Claro)', async ({ page, browser, request }) => {
  // Demo data through the API: two named dashboards, one protected.
  expect((await request.post('/api/admin/login', { data: { password: 'screenshots-only' } })).status()).toBe(
    204,
  )
  const { csrf_token } = await (await request.get('/api/admin/session')).json()
  const headers = { 'X-CSRF-Token': csrf_token as string }
  for (const [slug, title] of [
    ['acme', 'Acme'],
    ['initech', 'Initech Ops'],
  ]) {
    const res = await request.post('/api/admin/dashboards', { data: { slug, title }, headers })
    expect(res.status()).toBe(201)
    if (slug === 'initech') {
      const { id } = await res.json()
      await request.put(`/api/admin/dashboards/${id}/password`, {
        data: { password: 'demo-password' },
        headers,
      })
    }
  }

  await page.setViewportSize({ width: 1280, height: 720 })
  await prepare(page, 'ocean-light', 'flat')
  await page.goto('/admin/login')
  await page.getByTestId('login-password').fill('screenshots-only')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await page.locator('.app-toolbar button').filter({ hasText: 'Dashboard:' }).click()
  await page.locator('.dhx_menu-button').filter({ hasText: 'Manage dashboards' }).click()
  await expect(page.getByTestId('dashboards-panel')).toBeVisible()
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: `${OUT}/dashboards-panel-ocean-light.png`, animations: 'disabled' })

  // A visitor without a session sees the unlock screen.
  const visitor = await browser.newContext({ viewport: { width: 1280, height: 720 } })
  const visitorPage = await visitor.newPage()
  await prepare(visitorPage, 'ocean-light', 'flat')
  await visitorPage.goto('/dash/initech')
  await expect(visitorPage.getByTestId('unlock-form')).toBeVisible()
  await visitorPage.evaluate(() => document.fonts.ready)
  await visitorPage.screenshot({ path: `${OUT}/unlock-ocean-light.png`, animations: 'disabled' })
  await visitor.close()
})
