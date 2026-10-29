import { expect, test, type Page } from '@playwright/test'

import { isolateNetwork, openFromMenu, toolbarButton } from './helpers'

const SHOTS = process.env.E2E_SCREENSHOTS

/** Expected UI scale per resolution (see --ui-scale in theme.css). */
const SCREENS = [
  { name: 'Full HD', width: 1920, height: 1080, scale: 1 },
  { name: 'QHD', width: 2560, height: 1440, scale: 1.25 },
  { name: '4K', width: 3840, height: 2160, scale: 1.6 },
] as const

async function height(page: Page, selector: string): Promise<number> {
  return page
    .locator(selector)
    .first()
    .evaluate((el) => el.getBoundingClientRect().height)
}

async function noHorizontalOverflow(page: Page): Promise<void> {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  expect(overflow).toBeLessThanOrEqual(0)
}

for (const screen of SCREENS) {
  test(`dashboard and admin scale on ${screen.name} (${screen.width}x${screen.height})`, async ({ page }) => {
    await isolateNetwork(page)
    await page.setViewportSize({ width: screen.width, height: screen.height })
    await page.goto('/')

    await expect(toolbarButton(page, 'Development')).toBeVisible()
    expect(await height(page, '.dhx_toolbar.app-toolbar')).toBeCloseTo(32 * screen.scale, 0)
    expect(await height(page, '.app-tabbar .dhx_tabbar-tab')).toBeCloseTo(Math.round(26 * screen.scale), 0)
    await noHorizontalOverflow(page)

    await openFromMenu(page, 'Tests', 'Page A')
    await expect(page.locator('.pane:visible iframe')).toBeVisible()
    await noHorizontalOverflow(page)

    // Tabs are laid out side by side at the scaled size, never overlapping.
    const tabs = await page
      .locator('.app-tabbar .dhx_tabbar-tab')
      .evaluateAll((els) =>
        els.map((el) => el.getBoundingClientRect()).map((r) => ({ x: r.x, w: r.width, h: r.height })),
      )
    expect(tabs).toHaveLength(2)
    expect(Math.abs(tabs[0]!.w - 190 * screen.scale)).toBeLessThanOrEqual(1)
    expect(tabs[1]!.x).toBeGreaterThanOrEqual(tabs[0]!.x + tabs[0]!.w - 1)
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/dashboard-${screen.width}.png` })

    await page.goto('/admin/login')
    await page.getByTestId('login-password').fill('e2e-password-123')
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page).toHaveURL(/\/admin$/)
    await page
      .locator('.dhx_tree-list-item__text')
      .filter({ hasText: /^Gitea$/ })
      .click()

    // Action buttons (icon picker, copy URL) stay on their input's row, aligned with the input box.
    for (const [label, text] of [
      ['Icon (MDI class)', 'Choose…'],
      ['URL (empty', 'Copy'],
    ] as const) {
      const input = page
        .locator('.dhx_form-group')
        .filter({ hasText: label })
        .locator('.dhx_input__container')
      const button = page.locator('.field-action-button button').filter({ hasText: text })
      const [inputBox, buttonBox] = await Promise.all([input.boundingBox(), button.boundingBox()])
      expect(Math.abs(buttonBox!.y - inputBox!.y)).toBeLessThanOrEqual(1)
      expect(Math.abs(buttonBox!.height - inputBox!.height)).toBeLessThanOrEqual(1)
      expect(buttonBox!.x).toBeGreaterThan(inputBox!.x + inputBox!.width)
    }
    await noHorizontalOverflow(page)
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/admin-${screen.width}.png` })
  })
}

test('widgets rescale when the window crosses a breakpoint', async ({ page }) => {
  await isolateNetwork(page)
  await page.setViewportSize({ width: 1920, height: 1080 })
  await page.goto('/')
  await openFromMenu(page, 'Tests', 'Page A')
  const tabWidth = () =>
    page
      .locator('.app-tabbar .dhx_tabbar-tab')
      .first()
      .evaluate((el) => el.getBoundingClientRect().width)
  expect(Math.abs((await tabWidth()) - 190)).toBeLessThanOrEqual(1)

  await page.setViewportSize({ width: 3840, height: 2160 })
  await expect.poll(tabWidth).toBeGreaterThan(190 * 1.6 - 2)

  await page.goto('/admin/login')
  await page.getByTestId('login-password').fill('e2e-password-123')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(/\/admin$/)
  const treeWidth = () =>
    page
      .locator('.dhx_tree')
      .first()
      .evaluate((el) => el.closest('.dhx_layout-cell')!.getBoundingClientRect().width)
  expect(Math.abs((await treeWidth()) - 340 * 1.6)).toBeLessThanOrEqual(2)
  await page.setViewportSize({ width: 1920, height: 1080 })
  await expect.poll(treeWidth).toBeLessThan(345)
})

test('the Inter font license ships with the binary', async ({ request }) => {
  const response = await request.get('/licenses/inter-OFL.txt')
  expect(response.ok()).toBe(true)
  expect(await response.text()).toContain('SIL Open Font License')
})
