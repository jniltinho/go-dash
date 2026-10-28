import { t, type MessageKey, type MessageParams } from './i18n'

/**
 * Maps raw API error messages (English, part of the code) to the text shown
 * in the UI, in the current locale. Unknown messages fall back to a generic
 * text that still carries the original message for troubleshooting.
 */
const rules: ReadonlyArray<[RegExp, MessageKey, ((m: RegExpExecArray) => MessageParams)?]> = [
  [/^title is required$/, 'error.titleRequired'],
  [/^title must have at most (\d+) characters$/, 'error.titleTooLong', (m) => ({ max: m[1]! })],
  [/^target must be/, 'error.target'],
  [/^placement must be/, 'error.placement'],
  [/^invalid URL$/, 'error.invalidUrl'],
  [/^URL must use http or https$/, 'error.urlScheme'],
  [/^URL must be absolute$/, 'error.urlAbsolute'],
  [/^the dashboard's own origin cannot be opened in an iframe/, 'error.sameOrigin'],
  [/^parent (\d+) does not exist$/, 'error.parentMissing', (m) => ({ id: m[1]! })],
  [/^moving item (\d+) would create a cycle$/, 'error.cycle'],
  [/^item (\d+) does not exist$/, 'error.itemMissing', (m) => ({ id: m[1]! })],
  [/^item has (\d+) children/, 'error.hasChildren', (m) => ({ count: m[1]! })],
  [/^unsupported format/, 'error.format'],
  [/^unsupported version/, 'error.version'],
  [/^mode must be/, 'error.importMode'],
  [/^null item$/, 'error.nullItem'],
  [/^item not found$/, 'error.itemNotFound'],
  [/^authentication required$/, 'error.authRequired'],
  [/^invalid CSRF token$/, 'error.csrf'],
  [/^too many login attempts/, 'error.tooManyAttempts'],
  [/^invalid credentials$/, 'error.invalidCredentials'],
  [/^invalid request body$/, 'error.invalidRequest'],
  [/^Request Entity Too Large$/i, 'error.tooLarge'],
  [/^admin interface is disabled$/, 'error.adminDisabled'],
  [/^cross-origin request rejected$/, 'error.crossOrigin'],
  [/^expected application\/json$/, 'error.invalidRequest'],
  [/^invalid id$/, 'error.invalidId'],
  [/^dashboard locked$/, 'error.dashboardLocked'],
  [/^dashboard not found$/, 'error.dashboardNotFound'],
  [/^dashboard is not protected$/, 'error.notProtected'],
  [/^dashboard has items/, 'error.dashboardHasItems'],
  [/^invalid password$/, 'error.wrongPassword'],
  [/^too many attempts/, 'error.tooManyAttempts'],
  [/^password must have (\d+) to (\d+) bytes$/, 'error.passwordLength', (m) => ({ min: m[1]!, max: m[2]! })],
  [/^slug is required$/, 'error.slugRequired'],
  [/^slug must have/, 'error.slugInvalid'],
  [/^slug "([^"]+)" is reserved$/, 'error.slugReserved', (m) => ({ slug: m[1]! })],
  [/^slug "([^"]+)" is already in use$/, 'error.slugInUse', (m) => ({ slug: m[1]! })],
  [/^the default dashboard has no slug$/, 'error.defaultNoSlug'],
  [/^the default dashboard cannot be deleted$/, 'error.defaultUndeletable'],
  [/^invalid dashboard id$/, 'error.invalidDashboardId'],
  [/^not found$/, 'error.notFound'],
  [/^internal server error$/, 'error.internal'],
]

export function translateError(message: string): string {
  for (const [pattern, key, params] of rules) {
    const match = pattern.exec(message)
    if (match) return t(key, params?.(match))
  }
  return t('error.unknown', { message })
}
