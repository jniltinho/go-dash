import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig, type Plugin } from 'vite-plus'

/**
 * DHTMLX's suite.min.css @imports its icon font from cdn.dhtmlx.com. The app
 * must work offline under a 'self'-only CSP, so the import is removed here and
 * a local copy (src/vendor/dhx-icons) is imported in main.ts instead.
 */
function stripDhxCdnImport(): Plugin {
  const cdnImport = /@import\s+url\(["']?https:\/\/cdn\.dhtmlx\.com\/[^)]*\)\s*;?/g
  return {
    name: 'strip-dhx-cdn-import',
    enforce: 'pre',
    transform(code, id) {
      if (!id.includes('@dhtmlx/suite') || !id.split('?')[0]!.endsWith('.css')) return null
      return { code: code.replace(cdnImport, ''), map: null }
    },
  }
}

/**
 * Exposes the Material Design Icons names as `virtual:mdi-icons`, parsed from
 * the $mdi-icons map shipped with @mdi/font, so the icon picker always matches
 * the installed font. The module is only loaded when the picker opens.
 */
function mdiIconNames(): Plugin {
  const virtualId = 'virtual:mdi-icons'
  const resolvedId = '\0' + virtualId
  return {
    name: 'mdi-icon-names',
    resolveId(id) {
      return id === virtualId ? resolvedId : null
    },
    load(id) {
      if (id !== resolvedId) return null
      const file = fileURLToPath(new URL('./node_modules/@mdi/font/scss/_variables.scss', import.meta.url))
      const scss = readFileSync(file, 'utf8')
      const map = scss.slice(scss.indexOf('$mdi-icons:'))
      const names = [...map.matchAll(/^\s*"([a-z0-9-]+)":\s*F[0-9A-F]+/gm)].map((m) => m[1])
      if (names.length < 1000) throw new Error(`mdi-icon-names: parsed only ${names.length} icons`)
      return `export default ${JSON.stringify(names)}`
    },
  }
}

/**
 * The Go server injects <base href> into index.html. The dev server gets
 * <base href="/"> too, so relative URLs and basePath() behave the same on
 * deep routes such as /admin/login.
 */
function devBaseHref(): Plugin {
  return {
    name: 'dev-base-href',
    apply: 'serve',
    transformIndexHtml: (html) => html.replace(/<head>/i, '<head><base href="/" />'),
  }
}

const backend = process.env.DASHBOARD_BACKEND ?? 'http://localhost:8080'

export default defineConfig({
  // Relative asset URLs: one build works under any base path (see devBaseHref).
  base: './',
  plugins: [stripDhxCdnImport(), mdiIconNames(), devBaseHref(), vue(), tailwindcss()],
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 2500,
  },
  server: {
    port: 9000,
    proxy: {
      '/api': backend,
      '/healthz': backend,
    },
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'node',
  },
  lint: {
    ignorePatterns: ['dist/**', 'e2e/**', 'screenshots/**', 'playwright-report/**', 'test-results/**'],
  },
  fmt: {
    semi: false,
    singleQuote: true,
    printWidth: 110,
    ignorePatterns: ['playwright-report/**', 'test-results/**'],
  },
})
