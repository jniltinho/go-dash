import { expect, test, type Page } from '@playwright/test'

import { isolateNetwork, menuItem, toolbarButton, watchCSP } from './helpers'

const PASSWORD = 'e2e-password-123'

interface Node {
  title: string
  items: Node[]
}

async function login(page: Page) {
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/admin\/login$/)
  await page.getByTestId('login-password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.locator('.dhx_tree-list-item__text').first()).toBeVisible()
}

function treeItem(page: Page, text: string) {
  return page.locator('.dhx_tree-list-item__text').filter({ hasText: new RegExp(`^${text}`) })
}

function field(page: Page, label: string) {
  return page.locator('.dhx_form-group').filter({ hasText: label }).locator('input, textarea').first()
}

test('admin is disabled without a password', async ({ page }) => {
  await page.goto('http://127.0.0.1:8098/admin')
  await expect(page.getByTestId('admin-disabled')).toBeVisible()
  await expect(page).toHaveURL(/\/admin\/disabled$/)
})

test('login rejects a wrong password', async ({ page }) => {
  await page.goto('/admin/login')
  await page.getByTestId('login-password').fill('wrong-password')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('alert')).toHaveText('Invalid password.')
})

test('CRUD, field errors, import/export and logout', async ({ page }) => {
  await isolateNetwork(page)
  const csp = watchCSP(page)
  await login(page)

  // Create a root item.
  await toolbarButton(page, 'New').click()
  await field(page, 'Title').fill('Item E2E')
  await field(page, 'URL').fill('javascript:alert(1)')
  await toolbarButton(page, 'Save').click()
  await expect(page.locator('.dhx_form-group').filter({ hasText: 'URL' })).toContainText(
    'The URL must start with http:// or https://.',
  )

  await field(page, 'URL').fill('https://e2e.example.com')
  await toolbarButton(page, 'Save').click()
  await expect(treeItem(page, 'Item E2E')).toBeVisible()

  // Choose the icon visually in the picker modal (Portuguese terms work in any language).
  await page.locator('.dhx_form button').filter({ hasText: 'Choose' }).click()
  const picker = page.getByTestId('icon-picker')
  await expect(picker).toBeVisible()
  await expect(picker.locator('.icon-cell').first()).toBeVisible()
  await page.getByTestId('icon-search').fill('servidor')
  await picker.locator('.icon-cell[data-icon="server"]').click()
  await expect(page.getByTestId('icon-preview')).toContainText('mdi mdi-server')
  await picker.getByRole('button', { name: 'Use icon' }).click()
  await expect(picker).toHaveCount(0)
  await expect(field(page, 'Icon')).toHaveValue('mdi mdi-server')
  await page.locator('.dhx_form-group').filter({ hasText: 'Show in' }).locator('select').selectOption('menu')
  await toolbarButton(page, 'Save').click()
  await expect(page.locator('.dhx_message').filter({ hasText: 'Item saved' }).last()).toBeVisible()
  const saved = await page.request.get('/api/admin/items').then((r) => r.json())
  const savedItem = saved.find((n: Node & { icon: string }) => n.title === 'Item E2E')
  expect(savedItem?.icon).toBe('mdi mdi-server')
  expect(savedItem?.placement).toBe('menu')
  await expect(treeItem(page, 'Item E2E · menu only')).toBeVisible()

  // Keyboard: focus is trapped, arrows move, Enter uses the icon, Esc closes.
  const pickButton = page.locator('.dhx_form button').filter({ hasText: 'Choose' })
  await pickButton.click()
  await expect(page.getByTestId('icon-search')).toBeFocused()
  for (let i = 0; i < 8; i++) await page.keyboard.press('Tab')
  expect(await page.evaluate(() => !!document.activeElement?.closest('[role=dialog]'))).toBe(true)
  await page.getByTestId('icon-search').fill('house')
  await expect(picker.locator('.icon-cell[data-icon="home"]')).toBeVisible()
  await page.getByTestId('icon-search').press('ArrowDown')
  await expect(picker.locator('.icon-cell[data-icon="home"]')).toBeFocused()
  await page.keyboard.press('ArrowRight')
  const second = await picker.locator('.icon-cell').nth(1).getAttribute('data-icon')
  await expect(picker.locator('.icon-cell').nth(1)).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(field(page, 'Icon')).toHaveValue(`mdi mdi-${second}`)
  await pickButton.click()
  await page.keyboard.press('Escape')
  await expect(picker).toHaveCount(0)
  await expect(pickButton).toBeFocused()
  await field(page, 'Icon').fill('mdi mdi-server')

  // Child item under the selected one.
  await toolbarButton(page, 'New subitem').click()
  await field(page, 'Title').fill('Child E2E')
  await toolbarButton(page, 'Save').click()
  await expect(treeItem(page, 'Child E2E')).toBeVisible()

  // Rename.
  await treeItem(page, 'Child E2E').click()
  await field(page, 'Title').fill('Renamed child')
  await toolbarButton(page, 'Save').click()
  await expect(treeItem(page, 'Renamed child')).toBeVisible()

  // Reorder by drag and drop: move the child out of "Item E2E" onto "Maps".
  await treeItem(page, 'Renamed child').dragTo(treeItem(page, 'Maps'), { steps: 10 })
  await expect(page.locator('.dhx_message').filter({ hasText: 'Order saved' })).toBeVisible()
  const tree = await page.request.get('/api/admin/items').then((r) => r.json())
  const parentOf = (
    nodes: Node[],
    title: string,
    parent: string | null = null,
  ): string | null | undefined => {
    for (const n of nodes) {
      if (n.title === title) return parent
      const found = parentOf(n.items, title, n.title)
      if (found !== undefined) return found
    }
    return undefined
  }
  expect(parentOf(tree, 'Renamed child')).not.toBe('Item E2E')

  // Export.
  const [download] = await Promise.all([page.waitForEvent('download'), toolbarButton(page, 'Export').click()])
  expect(download.suggestedFilename()).toBe('dashboard-menu-default.json')

  // Import (append).
  await toolbarButton(page, 'Import').click()
  await page
    .locator('.dhx_window textarea')
    .fill(JSON.stringify({ format: 'dashboard-menu', version: 1, items: [{ title: 'Imported E2E' }] }))
  await page.locator('.dhx_window button').filter({ hasText: 'Import' }).click()
  await expect(treeItem(page, 'Imported E2E')).toBeVisible()

  // Delete with confirmation.
  await treeItem(page, 'Item E2E').click()
  await toolbarButton(page, 'Delete').click()
  await page.locator('[class*="dhx_alert"] button').filter({ hasText: 'Delete' }).click()
  await expect(treeItem(page, 'Item E2E')).toHaveCount(0)

  // Logout ends the session.
  await toolbarButton(page, 'Sign out').click()
  await expect(page).toHaveURL(/\/admin\/login$/)
  await page.goto('/admin')
  await expect(page).toHaveURL(/\/admin\/login$/)

  expect(csp).toEqual([])
})

for (const theme of [
  { name: 'matrix', onSelection: 'rgb(0, 0, 0)', onPrimary: 'rgb(0, 0, 0)' },
  { name: 'ocean-dark', onSelection: 'rgb(14, 31, 44)', onPrimary: 'rgb(14, 31, 44)' },
]) {
  test(`selection and checkboxes stay legible in ${theme.name}`, async ({ page }) => {
    await page.addInitScript(
      (value) => localStorage.setItem('dashboard.theme', value),
      JSON.stringify({ name: theme.name, style: '3d' }),
    )
    await login(page)
    await treeItem(page, 'Gitea').click()

    const text = treeItem(page, 'Gitea')
    await expect.poll(() => text.evaluate((el) => getComputedStyle(el).color)).toBe(theme.onSelection)

    const check = page.locator('.dhx_checkbox__visual-input').first()
    const mark = await check.evaluate((el) => getComputedStyle(el, '::after').borderRightColor)
    expect(mark).toBe(theme.onPrimary)
    expect(await check.evaluate((el) => getComputedStyle(el).backgroundImage)).toBe('none')
  })
}

test('duplicating an item and copying its URL', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await login(page)

  // Nothing selected: the copy needs a source item.
  await toolbarButton(page, 'Duplicate').click()
  await expect(page.locator('.dhx_message').last()).toHaveText('Select an item to duplicate')

  await treeItem(page, 'Gitea').click()
  await expect(field(page, 'Title')).toHaveValue('Gitea')
  const url = await field(page, 'URL').inputValue()
  expect(url).toMatch(/^https?:\/\//)

  await page.locator('.field-action-button button').filter({ hasText: 'Copy' }).click()
  await expect(page.locator('.dhx_message').last()).toHaveText('URL copied')
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(url)

  // "Duplicate" fills a new, unsaved item; "Save" creates it next to the original.
  await toolbarButton(page, 'Duplicate').click()
  await expect(field(page, 'Title')).toHaveValue('Gitea (copy)')
  await expect(field(page, 'URL')).toHaveValue(url)
  await expect(page.locator('input[name="context"]')).toHaveJSProperty(
    'value',
    'Copy of "Gitea" inside "Development" — adjust it and click Save',
  )
  await toolbarButton(page, 'Save').click()
  await expect(treeItem(page, 'Gitea \\(copy\\)')).toBeVisible()
  await expect(treeItem(page, 'Gitea')).toHaveCount(2)
  // Right after the original, before its next sibling.
  const order = await page.locator('.dhx_tree-list-item__text').allTextContents()
  expect(order.indexOf('Gitea (copy)')).toBe(order.indexOf('Gitea') + 1)

  type Saved = { title: string; url: string | null; items?: Saved[] }
  const flat = (nodes: Saved[]): Saved[] => nodes.flatMap((n) => [n, ...flat(n.items ?? [])])
  const saved: Saved[] = await page.request.get('/api/admin/items').then((r) => r.json())
  expect(flat(saved).find((n) => n.title === 'Gitea (copy)')?.url).toBe(url)

  // Clean up so other tests see the original menu.
  await toolbarButton(page, 'Delete').click()
  await page.locator('[class*="dhx_alert"] button').filter({ hasText: 'Delete' }).click()
  await expect(treeItem(page, 'Gitea')).toHaveCount(1)
})

test('the admin follows a language change without losing the edited item', async ({ page }) => {
  await login(page)
  await treeItem(page, 'Gitea').click()
  await field(page, 'Description').fill('unsaved text')

  await toolbarButton(page, 'Language').click()
  await menuItem(page, 'Português (Brasil)').click()
  await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR')
  await expect(toolbarButton(page, 'Salvar')).toBeVisible()
  await expect(page.getByText('Itens do menu')).toBeVisible()
  await expect(field(page, 'Título')).toHaveValue('Gitea')
  await expect(field(page, 'Descrição')).toHaveValue('unsaved text')
  const context = page.locator('input[name="context"]')
  await expect.poll(() => context.evaluate((el) => (el as HTMLInputElement).value)).toMatch(/^Editando #\d+/)
  const target = page.locator('.dhx_form-group').filter({ hasText: 'Abrir em' }).locator('select')
  await expect(target.locator('option:checked')).toHaveText('Nova aba do navegador')
})
