import '@dhtmlx/suite/codebase/suite.min.css'
import './vendor/dhx-icons/dhx-icons.css'
import '@mdi/font/css/materialdesignicons.css'
import './theme/main.css'

import { createPinia } from 'pinia'
import { createApp } from 'vue'

import { fetchConfigWithin } from './api/client'
import App from './App.vue'
import { applyLocale, initialLocale } from './lib/i18n'
import {
  applyTheme,
  DEFAULT_THEME,
  loadSavedThemePrefs,
  resolveThemePrefs,
  type ThemePrefs,
} from './lib/theme'
import { router } from './router'
import { CONFIG_TIMEOUT_MS, useConfigStore, useThemeStore } from './stores/app'

// One startup fetch of /api/config, bounded by a timeout, serves both the
// default theme and the config store (title, tab limit, iframe sandbox).
const config = await fetchConfigWithin(CONFIG_TIMEOUT_MS)

/**
 * Initial theme: the viewer's saved choice, otherwise the server default
 * (default_theme / default_theme_style), otherwise the built-in default.
 */
function initialTheme(): ThemePrefs {
  const saved = loadSavedThemePrefs()
  if (saved) return saved
  if (!config) return DEFAULT_THEME
  return resolveThemePrefs(config.default_theme, config.default_theme_style)
}

const theme = initialTheme()
// Applied before mounting so the first paint already uses the theme and language.
applyTheme(theme)
applyLocale(initialLocale())

const app = createApp(App).use(createPinia()).use(router)
useThemeStore().init(theme)
if (config) useConfigStore().set(config)
app.mount('#app')
