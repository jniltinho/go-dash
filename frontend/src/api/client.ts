import type {
  Dashboard,
  DashboardInput,
  DashboardRef,
  DashboardStatus,
  ImportMode,
  MenuInput,
  MenuItem,
  MenuNode,
  Move,
  PublicConfig,
  SessionStatus,
} from './types'
import { apiUrl } from '../lib/basePath'

/** Error returned by the API, carrying the HTTP status and the invalid field, if any. */
export class ApiError extends Error {
  readonly status: number
  readonly field?: string

  constructor(status: number, message: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.field = field
  }
}

/** Fetches the public config, giving up after timeoutMs (null on failure or timeout). */
export async function fetchConfigWithin(timeoutMs: number): Promise<PublicConfig | null> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    return await api.config(controller.signal)
  } catch {
    return null
  } finally {
    clearTimeout(timer)
  }
}

let csrfToken: string | null = null

/** Sets the CSRF token sent with every mutating admin request. */
export function setCsrfToken(token: string | null): void {
  csrfToken = token
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken

  const response = await fetch(apiUrl(path), {
    method,
    headers,
    credentials: 'same-origin',
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  })

  if (response.status === 204) return undefined as T

  const text = await response.text()
  const data = text ? JSON.parse(text) : undefined
  if (!response.ok) {
    throw new ApiError(response.status, data?.error ?? response.statusText, data?.field)
  }
  return data as T
}

const dash = (slug: string) => `/api/dash/${encodeURIComponent(slug)}`
const adminDash = (ref: DashboardRef) => `/api/admin/dashboards/${ref}`

export const api = {
  config: (signal?: AbortSignal) => request<PublicConfig>('GET', '/api/config', undefined, signal),
  /** Menu of a dashboard; slug "default" is the dashboard served at "/". */
  menu: (slug: string) => request<MenuNode[]>('GET', `${dash(slug)}/menu`),
  dashboardStatus: (slug: string) => request<DashboardStatus>('GET', `${dash(slug)}/status`),
  unlock: (slug: string, password: string) => request<void>('POST', `${dash(slug)}/unlock`, { password }),
  lock: (slug: string) => request<void>('POST', `${dash(slug)}/lock`),

  session: () => request<SessionStatus>('GET', '/api/admin/session'),
  login: (password: string) => request<void>('POST', '/api/admin/login', { password }),
  logout: () => request<void>('POST', '/api/admin/logout'),

  listDashboards: () => request<Dashboard[]>('GET', '/api/admin/dashboards'),
  createDashboard: (input: DashboardInput) => request<Dashboard>('POST', '/api/admin/dashboards', input),
  updateDashboard: (ref: DashboardRef, input: DashboardInput) =>
    request<Dashboard>('PUT', adminDash(ref), input),
  deleteDashboard: (ref: DashboardRef, force: boolean) =>
    request<void>('DELETE', `${adminDash(ref)}${force ? '?force=true' : ''}`),
  setDashboardPassword: (ref: DashboardRef, password: string) =>
    request<void>('PUT', `${adminDash(ref)}/password`, { password }),
  clearDashboardPassword: (ref: DashboardRef) => request<void>('DELETE', `${adminDash(ref)}/password`),

  adminTree: (ref: DashboardRef) => request<MenuNode[]>('GET', `${adminDash(ref)}/items`),
  createItem: (ref: DashboardRef, input: MenuInput) =>
    request<MenuItem>('POST', `${adminDash(ref)}/items`, input),
  updateItem: (ref: DashboardRef, id: number, input: MenuInput) =>
    request<MenuItem>('PUT', `${adminDash(ref)}/items/${id}`, input),
  deleteItem: (ref: DashboardRef, id: number) => request<void>('DELETE', `${adminDash(ref)}/items/${id}`),
  reorder: (ref: DashboardRef, moves: Move[]) =>
    request<void>('POST', `${adminDash(ref)}/items/reorder`, { moves }),
  exportMenu: (ref: DashboardRef) => request<unknown>('GET', `${adminDash(ref)}/export`),
  importMenu: (ref: DashboardRef, doc: unknown, mode: ImportMode) =>
    request<{ imported: number }>('POST', `${adminDash(ref)}/import?mode=${mode}`, doc),
}
