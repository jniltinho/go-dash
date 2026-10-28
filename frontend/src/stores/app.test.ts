import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vite-plus/test'

import type { MenuNode } from '../api/types'

const menu = vi.fn<(slug: string) => Promise<MenuNode[]>>()
vi.mock('../api/client', async (importActual) => {
  const actual = await importActual<typeof import('../api/client')>()
  return { ApiError: actual.ApiError, api: { menu: (slug: string) => menu(slug) } }
})

const { ApiError } = await import('../api/client')
const { useMenuStore } = await import('./app')

function root(id: number, title: string): MenuNode {
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
  }
}

/** A promise resolved by hand, to control the order responses arrive in. */
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('menu store refresh', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    menu.mockReset()
  })

  it('applies a changed menu and ignores an unchanged one', async () => {
    const store = useMenuStore()
    menu.mockResolvedValueOnce([root(1, 'A'), root(2, 'B')])
    await store.load('default')
    menu.mockResolvedValueOnce([root(1, 'A'), root(2, 'B')])
    expect(await store.refresh()).toBe(false)
    menu.mockResolvedValueOnce([root(2, 'B'), root(1, 'A')])
    expect(await store.refresh()).toBe(true)
    expect(store.tree.map((n) => n.id)).toEqual([2, 1])
    expect(store.byId.get(1)?.title).toBe('A')
  })

  it('never lets an older response overwrite a newer one', async () => {
    const store = useMenuStore()
    menu.mockResolvedValueOnce([root(1, 'A')])
    await store.load('default')
    const slow = deferred<MenuNode[]>()
    const fast = deferred<MenuNode[]>()
    menu.mockReturnValueOnce(slow.promise).mockReturnValueOnce(fast.promise)
    const first = store.refresh()
    const second = store.refresh()
    fast.resolve([root(1, 'New')])
    expect(await second).toBe(true)
    slow.resolve([root(1, 'Antiga')])
    expect(await first).toBe(false)
    expect(store.tree[0]!.title).toBe('New')
  })

  it('keeps the latest of two overlapping loads', async () => {
    const store = useMenuStore()
    const slow = deferred<MenuNode[]>()
    const fast = deferred<MenuNode[]>()
    menu.mockReturnValueOnce(slow.promise).mockReturnValueOnce(fast.promise)
    const first = store.load('default')
    const second = store.load('default')
    fast.resolve([root(1, 'New')])
    await second
    expect(store.loading).toBe(false)
    slow.resolve([root(1, 'Antiga')])
    await first
    expect(store.tree[0]!.title).toBe('New')
    expect(store.loading).toBe(false)
  })

  it('drops the response when applying is no longer allowed, and keeps the menu on errors', async () => {
    const store = useMenuStore()
    menu.mockResolvedValueOnce([root(1, 'A')])
    await store.load('default')
    menu.mockResolvedValueOnce([root(1, 'B')])
    expect(await store.refresh(() => false)).toBe(false)
    menu.mockRejectedValueOnce(new Error('offline'))
    expect(await store.refresh()).toBe(false)
    expect(store.tree[0]!.title).toBe('A')
    expect(store.error).toBeNull()
  })

  it('drops a late response of the previous dashboard', async () => {
    const store = useMenuStore()
    const old = deferred<MenuNode[]>()
    menu.mockReturnValueOnce(old.promise).mockResolvedValueOnce([root(2, 'Acme')])
    const first = store.load('initech')
    await store.load('acme')
    old.resolve([root(1, 'Segredo')])
    await first
    expect(store.slug).toBe('acme')
    expect(store.tree.map((n) => n.title)).toEqual(['Acme'])
    expect(menu).toHaveBeenLastCalledWith('acme')
  })

  it('clears the menu when access is refused, but keeps it on server errors', async () => {
    const store = useMenuStore()
    menu.mockResolvedValueOnce([root(1, 'Segredo')])
    await store.load('initech')

    menu.mockRejectedValueOnce(new ApiError(503, 'unavailable'))
    expect(await store.refresh()).toBe(false)
    expect(store.tree).toHaveLength(1)

    menu.mockRejectedValueOnce(new ApiError(401, 'dashboard locked'))
    expect(await store.refresh()).toBe(false)
    expect(store.tree).toEqual([])
    expect(store.byId.size).toBe(0)
    expect(store.access).toBe('locked')

    menu.mockRejectedValueOnce(new ApiError(404, 'dashboard not found'))
    await store.load('initech')
    expect(store.access).toBe('not-found')
    expect(store.error).toBeNull()
  })

  it('reset forgets the previous dashboard and ignores in-flight refreshes', async () => {
    const store = useMenuStore()
    menu.mockResolvedValueOnce([root(1, 'A')])
    await store.load('initech')
    const pending = deferred<MenuNode[]>()
    menu.mockReturnValueOnce(pending.promise)
    const refresh = store.refresh()
    store.reset('acme')
    pending.resolve([root(1, 'B')])
    expect(await refresh).toBe(false)
    expect(store.tree).toEqual([])
    expect(store.slug).toBe('acme')
  })
})
