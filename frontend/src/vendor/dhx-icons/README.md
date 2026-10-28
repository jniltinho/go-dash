# DHTMLX icons (dxi)

Local copy of `https://cdn.dhtmlx.com/fonts/dxi/dhx-icons.css` and its fonts
(`dhxicons.woff`, `dhxicons.woff2`), downloaded for DHTMLX Suite 9.3.x.

`@dhtmlx/suite/codebase/suite.min.css` imports this stylesheet from the CDN.
The dashboard must work offline and its CSP only allows `'self'`, so the
`stripDhxCdnImport` plugin in `vite.config.ts` removes that `@import` at build
time and `src/main.ts` imports this local copy instead.

When upgrading `@dhtmlx/suite`, re-download these files if the icon set changes.
