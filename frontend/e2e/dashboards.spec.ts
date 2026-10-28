import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

import { isolateNetwork, menuItem, ORIGIN, tab, toolbarButton, watchCSP } from './helpers'

const PASSWORD = 'e2e-password-123'

async function login(page: Page) {
  await page.goto('/admin/login')
  await page.getByTestId('login-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(/\/admin$/)
}

/** Admin API client that logs in on its own cookie jar. */
async function adminApi(request: APIRequestContext) {
  const login = await request.post('/api/admin/login', { data: { password: PASSWORD } })
  expect(login.status()).toBe(204)
  const session = await (await request.get('/api/admin/session')).json()
  const headers = { 'X-CSRF-Token': session.csrf_token as string }
  return {
    async createDashboard(slug: string, title: string): Promise<number> {
      const res = await request.post('/api/admin/dashboards', { data: { slug, title }, headers })
      expect(res.status(), await res.text()).toBe(201)
      return (await res.json()).id
    },
    async addItem(dash: number | 'default', title: string, url: string) {
      const res = await request.post(`/api/admin/dashboards/${dash}/items`, {
        data: { title, url, target: 'iframe' },
        headers,
      })
      expect(res.status(), await res.text()).toBe(201)
    },
    async setPassword(dash: number | 'default', password: string) {
      const res = await request.put(`/api/admin/dashboards/${dash}/password`, { data: { password }, headers })
      expect(res.status(), await res.text()).toBe(204)
    },
    async clearPassword(dash: number | 'default') {
      const res = await request.delete(`/api/admin/dashboards/${dash}/password`, { headers })
      expect(res.status(), await res.text()).toBe(204)
    },
  }
}

async function unlock(page: Page, password: string) {
  await page.getByTestId('unlock-password').fill(password)
  await page.getByTestId('unlock-submit').click()
}

test('a dashboard created in the admin gets its own title, menu and address', async ({ page }) => {
  await isolateNetwork(page)
  const csp = watchCSP(page)
  await login(page)

  await toolbarButton(page, 'Dashboard: Go Dash (/)').click()
  await menuItem(page, 'Manage dashboards…').click()
  const panel = page.getByTestId('dashboards-panel')
  await expect(panel).toBeVisible()

  await panel.getByTestId('dash-title').fill('Globex Prices')
  await panel.getByTestId('dash-slug').fill('Admin')
  await expect(panel.getByTestId('dash-slug-hint')).toHaveText('"admin" is reserved.')
  await panel.getByTestId('dash-save').click()
  await expect(panel.getByTestId('dash-error')).toHaveText('The address "admin" is reserved.')

  await panel.getByTestId('dash-slug').fill('Globex-E2E')
  await expect(panel.getByTestId('dash-slug-hint')).toHaveText('Address: /dash/globex-e2e')
  await panel.getByTestId('dash-save').click()
  await expect(panel.getByTestId('dash-row-globex-e2e')).toContainText('Globex Prices')
  await page.keyboard.press('Escape')
  await expect(panel).toBeHidden()

  // Items are added to the selected dashboard only.
  await toolbarButton(page, 'Dashboard: Go Dash (/)').click()
  await menuItem(page, 'Globex Prices (/dash/globex-e2e)').click()
  await expect(toolbarButton(page, 'Dashboard: Globex Prices (/dash/globex-e2e)')).toBeVisible()
  await toolbarButton(page, 'New').click()
  const title = page.locator('.dhx_form-group').filter({ hasText: 'Title' }).locator('input').first()
  await title.fill('Globex Table')
  await page
    .locator('.dhx_form-group')
    .filter({ hasText: 'URL' })
    .locator('input')
    .first()
    .fill('https://globex.e2e.test')
  await toolbarButton(page, 'Save').click()
  await expect(page.locator('.dhx_tree-list-item__text').filter({ hasText: 'Globex Table' })).toBeVisible()

  // "Dashboard" opens the dashboard being edited, with its own title.
  await toolbarButton(page, 'Dashboard').click()
  await expect(page).toHaveURL(/\/dash\/globex-e2e$/)
  await expect(page.locator('.app-toolbar .dhx_navbar-title').first()).toHaveText('Globex Prices')
  await expect(page).toHaveTitle('Globex Prices')
  await toolbarButton(page, 'Globex Table').click()
  await expect(tab(page, 'Globex Table')).toBeVisible()

  // The default dashboard keeps its own title and does not show the item.
  await page.goto('/')
  await expect(page.locator('.app-toolbar .dhx_navbar-title').first()).toHaveText('Go Dash')
  await expect(toolbarButton(page, 'Globex Table')).toHaveCount(0)
  expect(csp).toEqual([])
})

test('dashboard addresses are canonical and unknown ones say so', async ({ page }) => {
  await page.goto('/dash/Globex-E2E/')
  await expect(page).toHaveURL(`${ORIGIN}/dash/globex-e2e`)
  await page.goto('/dash/default')
  await expect(page).toHaveURL(`${ORIGIN}/`)

  await page.goto('/dash/does-not-exist')
  await expect(page.getByTestId('dashboard-not-found')).toBeVisible()
  await expect(page).toHaveTitle('Dashboard not found')
  await page.getByRole('link', { name: 'Go to Home' }).click()
  await expect(page).toHaveURL(`${ORIGIN}/`)
})

test('each protected dashboard asks for its own password', async ({ page, browser, request }) => {
  await isolateNetwork(page)
  const admin = await adminApi(request)
  const over = await admin.createDashboard('initech-e2e', 'Initech')
  const bio = await admin.createDashboard('acme-e2e', 'Acme')
  await admin.addItem(over, 'Secret Panel', 'https://secret.e2e.test')
  await admin.setPassword(over, 'initech-secret')
  await admin.setPassword(bio, 'acme-secret')

  await page.goto('/dash/initech-e2e')
  await expect(page.getByTestId('unlock-title')).toHaveText('Initech')
  await expect(page.getByText('Secret Panel')).toHaveCount(0)

  await unlock(page, 'wrong-password')
  await expect(page.getByTestId('unlock-error')).toHaveText('Wrong password.')

  await unlock(page, 'initech-secret')
  await toolbarButton(page, 'Secret Panel').click()
  await expect(tab(page, 'Secret Panel')).toBeVisible()
  await page.reload()
  await expect(toolbarButton(page, 'Secret Panel')).toBeVisible()

  // The initech session does not open acme.
  await page.goto('/dash/acme-e2e')
  await expect(page.getByTestId('unlock-title')).toHaveText('Acme')

  // A password change locks open tabs out: tabs and iframes are discarded.
  await page.goto('/dash/initech-e2e')
  await toolbarButton(page, 'Secret Panel').click()
  await expect(page.getByTestId('item-iframe')).toHaveCount(1)
  await admin.setPassword(over, 'initech-new-secret')
  await page.evaluate(() => window.dispatchEvent(new Event('focus')))
  await expect(page.getByTestId('unlock-form')).toBeVisible()
  await expect(page.getByTestId('item-iframe')).toHaveCount(0)
  await expect(page.getByText('Secret Panel')).toHaveCount(0)

  // "Lock" ends the viewer session.
  await unlock(page, 'initech-new-secret')
  await toolbarButton(page, 'Lock').click()
  await expect(page.getByTestId('unlock-form')).toBeVisible()
  await page.reload()
  await expect(page.getByTestId('unlock-form')).toBeVisible()

  // A logged-in admin opens protected dashboards without their password.
  const adminContext = await browser.newContext()
  const adminPage = await adminContext.newPage()
  await isolateNetwork(adminPage)
  await login(adminPage)
  await adminPage.goto('/dash/initech-e2e')
  await expect(toolbarButton(adminPage, 'Secret Panel')).toBeVisible()
  await adminContext.close()
})

test('the default dashboard can be protected too', async ({ page, request }) => {
  const admin = await adminApi(request)
  await admin.setPassword('default', 'default-secret')
  try {
    await page.goto('/')
    await expect(page.getByTestId('unlock-title')).toHaveText('Go Dash')
    await unlock(page, 'default-secret')
    await expect(page.getByTestId('dashboard')).toBeVisible()
    await expect(toolbarButton(page, 'Lock')).toBeVisible()
  } finally {
    await admin.clearPassword('default')
  }
  await page.reload()
  await expect(toolbarButton(page, 'Lock')).toHaveCount(0)
})

/** Distance from the viewport top as a fraction of its height. */
async function topRatio(page: Page, selector: string): Promise<number> {
  const element = page.locator(selector).first()
  // DHTMLX popups slide in (translateY): measure where they settle.
  await element.evaluate((el) => Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished)))
  const box = await element.boundingBox()
  const height = page.viewportSize()!.height
  return box!.y / height
}

test('every modal opens 10% below the top', async ({ page }) => {
  await login(page)

  await toolbarButton(page, 'Dashboard: Go Dash (/)').click()
  await menuItem(page, 'Manage dashboards…').click()
  expect(await topRatio(page, '[data-testid=dashboards-panel] [role=dialog]')).toBeCloseTo(0.1, 2)
  await page.keyboard.press('Escape')

  await toolbarButton(page, 'New').click()
  await page.locator('.field-action-button button').filter({ hasText: 'Choose' }).click()
  expect(await topRatio(page, '[data-testid=icon-picker] [role=dialog]')).toBeCloseTo(0.1, 2)
  await page.keyboard.press('Escape')

  await toolbarButton(page, 'Import').click()
  expect(await topRatio(page, '.dhx_popup--window_modal')).toBeCloseTo(0.1, 2)
  await page.locator('.dhx_window').getByRole('button', { name: 'Cancel' }).click()

  await page.locator('.dhx_tree-list-item__text').first().click()
  await toolbarButton(page, 'Delete').click()
  expect(await topRatio(page, '.dhx_alert')).toBeCloseTo(0.1, 2)
  await page.locator('.dhx_alert__confirm-reject').click()

  // Logged out, the login card sits at the same height.
  await page.context().clearCookies()
  await page.goto('/admin/login')
  expect(await topRatio(page, '[data-testid=login-form]')).toBeCloseTo(0.1, 2)
})
