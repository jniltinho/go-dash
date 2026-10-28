import { expect, type Page } from '@playwright/test'

export const ORIGIN = 'http://127.0.0.1:8099'

const pages: Record<string, string> = {
  'top.e2e.test': `<!doctype html><title>top</title><p>trying to hijack</p>
    <script>try { top.location.href = 'https://example.com/hijacked' } catch (e) { document.body.dataset.blocked = '1' }</script>`,
}

/**
 * Serves fake pages for every external host so tests never hit the network,
 * and records requests the dashboard page itself makes to other origins.
 */
export async function isolateNetwork(page: Page): Promise<string[]> {
  const external: string[] = []
  await page.route('**/*', async (route) => {
    const url = new URL(route.request().url())
    if (url.origin === ORIGIN || url.origin === 'http://127.0.0.1:8098') return route.continue()
    if (route.request().frame() === page.mainFrame()) external.push(url.href)
    const body =
      pages[url.hostname] ?? `<!doctype html><title>${url.hostname}</title><input id="state" value="">`
    return route.fulfill({ status: 200, contentType: 'text/html', body })
  })
  return external
}

/** Collects CSP violations reported by the browser. */
export function watchCSP(page: Page): string[] {
  const violations: string[] = []
  page.on('console', (msg) => {
    if (/Content Security Policy/i.test(msg.text())) violations.push(msg.text())
  })
  return violations
}

function exact(text: string): RegExp {
  return new RegExp(`^\\s*${text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*$`)
}

/**
 * Toolbar buttons and menu items are matched by their text node: Chrome adds
 * the MDI icon glyph (CSS ::before) to the accessible name.
 */
export function toolbarButton(page: Page, text: string) {
  return page.locator('.app-toolbar > .dhx_navbar button').filter({ hasText: exact(text) })
}

export function menuItem(page: Page, text: string) {
  return page
    .locator('.dhx_menu-button')
    .filter({ has: page.locator('.dhx_menu-button__text', { hasText: exact(text) }) })
}

/** Closes open dropdowns by clicking the (non-interactive) toolbar title. */
export async function closeMenus(page: Page) {
  await page.locator('.app-toolbar .dhx_navbar-title').first().click()
}

export function tab(page: Page, text: string) {
  return page.locator('.dhx_tabbar-tab').filter({ hasText: text })
}

export async function openFromMenu(page: Page, ...path: string[]) {
  await toolbarButton(page, path[0]!).click()
  for (const entry of path.slice(1)) {
    await menuItem(page, entry).click()
  }
}

/** Theme labels grouped as in the "Theme" menu. */
const THEME_GROUP: Record<string, string> = {
  'Blue and Black': 'Dark',
  Dark: 'Dark',
  Graphite: 'Dark',
  Midnight: 'Dark',
  Light: 'Light',
  'Ocean Light': 'Brand',
  'Ocean Dark': 'Brand',
  'Bio Manager': 'Brand',
  Google: 'Inspired',
  Microsoft: 'Inspired',
  'Super Mario Galaxy': 'Inspired',
  Brasil: 'Inspired',
  'Windows XP': 'Systems',
  'Windows 98': 'Systems',
  'Classic Mac OS': 'Systems',
  Winamp: 'Inspired',
  'Windows Vista': 'Systems',
  'Windows 7': 'Systems',
  'Windows 10': 'Systems',
  'Linux KDE': 'Systems',
  Ubuntu: 'Systems',
  macOS: 'Systems',
  xterm: 'Systems',
  Matrix: 'Special',
  'High Contrast': 'Special',
}

export async function setTheme(page: Page, theme: string, style: string) {
  await toolbarButton(page, 'Theme').click()
  const group = THEME_GROUP[theme]!
  await menuItem(page, group).click()
  // Some themes share their group's name ("Dark", "Light"): pick the entry in the group's submenu.
  await page
    .locator(`.dhx_widget.dhx_menu[aria-label="${group}"] .dhx_menu-button`)
    .filter({ has: page.locator('.dhx_menu-button__text', { hasText: exact(theme) }) })
    .click()
  await toolbarButton(page, 'Theme').click()
  await menuItem(page, style).click()
}

export async function expectNoRoundedCorners(page: Page) {
  const rounded = await page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>('body *')]
      .filter((el) => el.offsetParent !== null)
      .filter((el) => parseFloat(getComputedStyle(el).borderTopLeftRadius) > 0)
      .map((el) => `${el.tagName}.${el.className}`)
      .slice(0, 5),
  )
  expect(rounded).toEqual([])
}
