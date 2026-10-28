/**
 * The base path the application is served under ("/" or e.g. "/panel/").
 * The Go server injects it into index.html as <base href>, so one build works
 * under any prefix. Root-absolute URLs ("/api/...", href="/admin") must never
 * be written by hand: use apiUrl() for requests and the router for links.
 */

/**
 * Resolves the base path from the href of the <base> element. Without one
 * (no DOM, or a page not served by the Go server) it is "/" — never the
 * current route.
 */
export function basePathFrom(baseHref: string | null, documentUrl: string): string {
  if (!baseHref) return '/'
  try {
    const path = new URL(baseHref, documentUrl).pathname
    return path.endsWith('/') ? path : `${path}/`
  } catch {
    return '/'
  }
}

export function basePath(): string {
  if (typeof document === 'undefined') return '/'
  return basePathFrom(document.querySelector('base[href]')?.getAttribute('href') ?? null, document.URL)
}

/** URL of an API path ("/api/menu") under the base path. */
export function apiUrl(path: string, base = basePath()): string {
  return base + path.replace(/^\/+/, '')
}

/**
 * localStorage key for a per-viewer convenience. Unchanged at "/", prefixed
 * with the base path otherwise, so instances on the same origin keep their
 * own preferences.
 */
export function storageKey(name: string, base = basePath()): string {
  return base === '/' ? name : `${base}${name}`
}
