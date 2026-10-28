import { expect, test, type Page } from '@playwright/test'

import { toolbarButton, watchCSP } from './helpers'

const PASSWORD = 'e2e-password-123'
const BASE = '/panel/'

/** The same instance (served under /panel/) reached with the prefix kept and stripped. */
const ORIGINS = [
  { name: 'prefix kept', origin: 'http://127.0.0.1:8096' },
  { name: 'prefix stripped by the proxy', origin: 'http://127.0.0.1:8095' },
]

/**
 * Fakes external sites and records every request of the page itself that
 * leaves the base path (iframes are excluded: they load external targets).
 */
async function isolate(page: Page, origin: string): Promise<string[]> {
  const outside: string[] = []
  await page.route('**/*', async (route) => {
    const url = new URL(route.request().url())
    if (url.origin === origin) {
      if (route.request().frame() === page.mainFrame() && !url.pathname.startsWith(BASE))
        outside.push(url.href)
      return route.continue()
    }
    return route.fulfill({ status: 200, contentType: 'text/html', body: '<!doctype html><title>x</title>' })
  })
  return outside
}

async function login(page: Page, origin: string) {
  await page.goto(`${origin}${BASE}admin/login`)
  await page.getByTestId('login-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(`${origin}${BASE}admin`)
}

for (const { name, origin } of ORIGINS) {
  test.describe(`under ${BASE} (${name})`, () => {
    test('deep links, navigation, hash and back stay under the base path', async ({ page }) => {
      const outside = await isolate(page, origin)
      const csp = watchCSP(page)

      await page.goto(`${origin}${BASE}`)
      await expect(page.locator('.app-toolbar .dhx_navbar-title').first()).toHaveText('Go Dash')

      // Opening a card puts #item-N on the prefixed URL; back returns home.
      await page.locator('.link-item').filter({ hasText: 'OpenStreetMap — São Paulo' }).first().click()
      await expect(page).toHaveURL(new RegExp(`^${origin}${BASE}#item-\\d+$`))
      await page.goBack()
      await expect(page).toHaveURL(`${origin}${BASE}`)

      await toolbarButton(page, 'Admin').click()
      await expect(page).toHaveURL(`${origin}${BASE}admin/login`)

      await page.goto(`${origin}${BASE}dash/default`)
      await expect(page).toHaveURL(`${origin}${BASE}`)
      expect(outside).toEqual([])
      expect(csp).toEqual([])

      // Typed without the trailing slash: redirected to the base path.
      await page.goto(`${origin}/panel`)
      await expect(page).toHaveURL(`${origin}${BASE}`)
    })

    test('admin, icon picker, cookies and conditional index work', async ({ page, context }) => {
      const outside = await isolate(page, origin)
      const csp = watchCSP(page)
      await login(page, origin)

      const cookies = await context.cookies(`${origin}${BASE}`)
      expect(cookies.find((c) => c.name === 'dashboard_session')?.path).toBe(BASE)

      // The lazy icon chunk and the MDI font load from under the prefix.
      await toolbarButton(page, 'New').click()
      await page.locator('.field-action-button button').filter({ hasText: 'Choose' }).click()
      await page.getByTestId('icon-search').fill('server')
      await expect(page.locator('.icon-cell[data-icon="server"]')).toBeVisible()
      await page.keyboard.press('Escape')

      await toolbarButton(page, 'Dashboard').click()
      await expect(page).toHaveURL(`${origin}${BASE}`)

      const index = await page.request.get(`${origin}${BASE}`)
      expect(await index.text()).toContain(`<base href="${BASE}" />`)
      const again = await page.request.get(`${origin}${BASE}`, {
        headers: { 'If-None-Match': index.headers()['etag']! },
      })
      expect(again.status()).toBe(304)

      expect(outside).toEqual([])
      expect(csp).toEqual([])
    })

    test('an empty dashboard links to the admin, and a protected one unlocks', async ({ page }) => {
      await isolate(page, origin)
      await login(page, origin)
      const { csrf_token } = await (await page.request.get(`${origin}${BASE}api/admin/session`)).json()
      const headers = { 'X-CSRF-Token': csrf_token as string }
      const slug = name.startsWith('prefix kept') ? 'empty-kept' : 'empty-stripped'
      const res = await page.request.post(`${origin}${BASE}api/admin/dashboards`, {
        data: { slug, title: 'Empty' },
        headers,
      })
      expect(res.status()).toBe(201)
      const { id } = await res.json()

      await page.goto(`${origin}${BASE}dash/${slug}`)
      await page.getByRole('link', { name: 'Admin' }).click()
      await expect(page).toHaveURL(`${origin}${BASE}admin`)

      expect(
        (
          await page.request.put(`${origin}${BASE}api/admin/dashboards/${id}/password`, {
            data: { password: 'empty-secret' },
            headers,
          })
        ).status(),
      ).toBe(204)

      // A visitor (no admin cookie) unlocks it under the prefix.
      await page.context().clearCookies()
      await page.goto(`${origin}${BASE}dash/${slug}`)
      await page.getByTestId('unlock-password').fill('empty-secret')
      await page.getByTestId('unlock-submit').click()
      await expect(page.getByTestId('dashboard')).toBeVisible()
      const viewer = (await page.context().cookies(`${origin}${BASE}`)).find((c) =>
        c.name.startsWith('dashboard_view_'),
      )
      expect(viewer?.path).toBe(BASE)
    })
  })
}
