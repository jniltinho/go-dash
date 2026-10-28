# dashboard-ui Specification

## Purpose
TBD - created by archiving change add-link-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Vue SPA with DHTMLX widgets
The interface SHALL be a Vue 3 + TypeScript SPA built with Vite+, in which each DHTMLX widget (Layout, Toolbar, Tabbar, Tree, Form) is wrapped in a Vue component that creates the instance on mount and destroys it (`destructor()`) on unmount.

#### Scenario: Navigate between views
- **WHEN** the administrator navigates from `/` to `/admin` and back to `/` within the SPA
- **THEN** every DHTMLX widget of the previous view is destroyed and the dashboard again shows a working toolbar and Tabbar, without duplicated instances

### Requirement: Page structure with DHTMLX Layout
The main page and the administration SHALL be structured with the DHTMLX Layout component (`dhx.Layout`) filling the whole viewport: a header cell with the Toolbar attached and a content cell taking all the remaining height. On the main page, the content cell MUST contain the Tabbar; in the administration, it MUST be split into resizable columns with the item tree and the form.

#### Scenario: Full viewport layout
- **WHEN** the browser window is resized
- **THEN** the header keeps its height and the content cell (Tabbar and active iframe) fills all the remaining area, without scrollbars on the main page

#### Scenario: Admin resizable columns
- **WHEN** the administrator drags the divider between the tree and the form
- **THEN** the column widths are adjusted and both components resize

### Requirement: Header with menu and submenus
The main page SHALL show a fixed header bar containing the dashboard title/logo and a DHTMLX Toolbar component (`dhx.Toolbar`) built from `GET /api/menu`, with nested submenus reflecting the stored hierarchy.

#### Scenario: Render nested menus
- **WHEN** there is "Monitoring" with children "Grafana" and "Zabbix"
- **THEN** the header shows "Monitoring" and, when it is clicked, shows the dropdown with "Grafana" and "Zabbix"

#### Scenario: Three levels deep
- **WHEN** there is "Infra" > "Databases" > "pgAdmin"
- **THEN** clicking "Infra" opens a dropdown with "Databases", which opens a sub-dropdown containing "pgAdmin", and clicking "pgAdmin" runs its action

#### Scenario: Title rendered as text
- **WHEN** an item has `title="<img src=x onerror=alert(1)>"`
- **THEN** the text is shown literally in the toolbar and no script is executed

#### Scenario: Menu API failure
- **WHEN** `GET /api/menu` fails
- **THEN** the toolbar shows only the title and the fixed actions, and the central area shows the message "Could not load the menu" and a "Try again" button that repeats the request

#### Scenario: Fixed toolbar actions
- **WHEN** the main page is loaded
- **THEN** the toolbar shows on the left the dashboard title followed by the menus and, on the right, the "Home" and "Admin" buttons

#### Scenario: Home button
- **WHEN** the user clicks "Home" in the toolbar
- **THEN** the "Home" tab is activated, the other tabs remain open and the URL hash is removed

### Requirement: Open link in central iframe
When an item with `url` and `target="iframe"` is clicked, the page SHALL load the URL in an iframe inside a Tabbar tab in the central area below the header, without reloading the main page. The iframe MUST use the configured `sandbox` attribute (default `allow-scripts allow-same-origin allow-forms allow-popups`, without `allow-top-navigation`, `allow-popups-to-escape-sandbox`, `allow-downloads` or `allow-modals`) and `referrerpolicy="no-referrer"`. The page MUST refuse to load in the iframe URLs with the same origin as the dashboard.

#### Scenario: Open in iframe
- **WHEN** the user clicks "Grafana" (`target="iframe"`)
- **THEN** a "Grafana" tab is created and activated in the Tabbar, its iframe loads the Grafana URL and the context bar shows the title "Grafana"

#### Scenario: Iframe cannot navigate top window
- **WHEN** the iframe content tries to redirect the main window (`top.location`)
- **THEN** the navigation is blocked and the dashboard remains loaded

#### Scenario: Same-origin URL refused in iframe
- **WHEN** the page receives an `iframe` item whose URL has the same origin as `location.origin`
- **THEN** the URL is not loaded in the iframe and the message "Open this item in a new tab" is shown

#### Scenario: Deep link via hash
- **WHEN** the user opens the page with `#item-<id>` of an existing `iframe` item
- **THEN** the page directly opens and activates that item's tab

#### Scenario: Invalid deep link
- **WHEN** the user opens `#item-<id>` of a nonexistent, disabled or URL-less item
- **THEN** the "Home" tab is activated and an "Item not found" message is shown

#### Scenario: Deep link to blank item
- **WHEN** the user opens `#item-<id>` of an item with `target="blank"`
- **THEN** no browser tab is opened automatically and the "Home" tab is shown with the item highlighted

#### Scenario: Browser back
- **WHEN** the user opens "Grafana" and then "Zabbix" and uses the browser back button
- **THEN** the "Grafana" tab becomes active again

### Requirement: Open link in new blank tab
When an item with `url` and `target="blank"` is clicked, the page SHALL open the URL in a new tab/window with `noopener` and `noreferrer`, keeping the current central content unchanged.

#### Scenario: Open in new tab
- **WHEN** the user clicks "Wiki" (`target="blank"`)
- **THEN** a new browser tab is opened with the Wiki URL and the Tabbar is not changed

### Requirement: Tabbed central area
The central area SHALL use the DHTMLX Tabbar component (`dhx.Tabbar`), with a fixed, non-closable "Home" tab containing the home page and one closable tab per opened `iframe` item. Each item MUST have at most one tab. Inactive tabs MUST keep their iframes loaded. The number of item tabs MUST be limited by `max_tabs` (default 10); when exceeded, the least recently used tab MUST be closed automatically with a notice to the user.

#### Scenario: Multiple tabs
- **WHEN** the user opens "Grafana" and then "Zabbix"
- **THEN** the Tabbar shows the tabs "Home", "Grafana" and "Zabbix", with "Zabbix" active

#### Scenario: Reuse existing tab
- **WHEN** "Grafana" is already open in a tab and the user clicks "Grafana" in the menu again
- **THEN** the existing tab is activated, no new tab is created and the iframe is not reloaded

#### Scenario: Preserve state of inactive tab
- **WHEN** the user navigates inside the "Grafana" iframe, switches to "Zabbix" and back to "Grafana"
- **THEN** the "Grafana" iframe shows the same page the user was on

#### Scenario: Close tab
- **WHEN** the user closes the active "Zabbix" tab
- **THEN** the tab and its iframe are removed and a neighbouring tab is activated

#### Scenario: Home tab cannot be closed
- **WHEN** the Tabbar is shown
- **THEN** the "Home" tab has no close control

#### Scenario: Tab limit
- **WHEN** `max_tabs` item tabs are open and the user opens a new item
- **THEN** the least recently used tab is closed, the new tab is opened and a message reports the closing

#### Scenario: Active tab in URL
- **WHEN** the user activates the "Grafana" tab by clicking it
- **THEN** `location.hash` becomes `#item-<Grafana id>`

### Requirement: Iframe fallback actions
Each item tab SHALL show, above its iframe, a context bar with the "Open in new tab" and "Reload" buttons, allowing the user to work around sites that block being shown in an iframe.

#### Scenario: Site blocks framing
- **WHEN** the site loaded in the iframe refuses to be shown because of `X-Frame-Options` and the user clicks "Open in new tab"
- **THEN** the same URL is opened in a new tab

### Requirement: Group items without URL
Items without `url` SHALL work only as submenu groups. Items with `url` that also have children SHALL show in the submenu an additional "Open <title>" entry that opens the item's own URL.

#### Scenario: Click group item
- **WHEN** the user clicks "Monitoring" (without URL)
- **THEN** only the submenu is shown and the Tabbar is not changed

#### Scenario: Group with URL
- **WHEN** "Grafana" has a URL and the children "Dashboards" and "Alerts", and the user opens its dropdown
- **THEN** the dropdown shows "Open Grafana" as the first entry, followed by "Dashboards" and "Alerts", and clicking "Open Grafana" opens the "Grafana" URL according to its `target`

### Requirement: Home page
The "Home" tab SHALL show a home page with cards grouping the top-level items and their clickable links, respecting each item's `target`.

#### Scenario: Initial load
- **WHEN** the user opens `/` without a hash
- **THEN** the Tabbar shows only the "Home" tab, active, with the link cards

### Requirement: Admin interface
The system SHALL offer at `/admin` a DHTMLX interface, accessible only after login, with a tree of the items (including disabled ones), an edit form (title, URL, target, icon, description, enabled), create/save/delete actions, reordering and reparenting by drag and drop, and JSON import/export.

#### Scenario: Drag to reorder
- **WHEN** the administrator drags "Zabbix" before "Grafana"
- **THEN** the new order is persisted and reflected in the public menu after reloading

#### Scenario: Validation feedback
- **WHEN** the administrator saves an item with an invalid URL
- **THEN** the form highlights the URL field with the error message returned by the API

#### Scenario: Admin not logged in
- **WHEN** a visitor without a session opens `/admin`
- **THEN** they are redirected to `/admin/login`

#### Scenario: Admin disabled
- **WHEN** no administrator password is configured and someone opens `/admin`
- **THEN** the SPA redirects to `/admin/disabled`, which states that web administration is disabled and how to enable it

### Requirement: Offline assets
All interface assets (HTML, JS, CSS, DHTMLX Suite, icons and fonts) SHALL be served by the binary itself, without requests to CDNs or external domains.

#### Scenario: Air-gapped network
- **WHEN** the dashboard is accessed on a network without internet access
- **THEN** the main page, the toolbar with its dropdowns, the login and the administration load without any request to hosts other than the dashboard's origin (excluding the destination URLs opened by the user)

### Requirement: Blue and black theme without rounded corners
The whole interface (main page, toolbar, dropdowns, Tabbar, home page, login and administration) SHALL use, by default, a custom theme with a black background and a blue primary color, implemented by overriding DHTMLX CSS variables without modifying the vendor files. No interface element MUST have rounded corners (computed `border-radius` equal to `0`). Text MUST have a minimum contrast of 7:1 (WCAG AAA) against the background in every state (normal, hover, active, pressed; disabled not included), according to the requirement "Theme registry with WCAG AAA contrast".

#### Scenario: Toolbar theme
- **WHEN** the main page is loaded
- **THEN** the toolbar has a black background with light text and the hovered or active item is highlighted in blue

#### Scenario: Tabbar theme
- **WHEN** there are open tabs in the Tabbar
- **THEN** the tab bar is black, inactive tabs use the dark surface and the active tab is highlighted in blue

#### Scenario: No rounded corners
- **WHEN** a browser test goes through the visible elements of the toolbar, dropdowns, Tabbar tabs, buttons, inputs, cards, messages and windows
- **THEN** all of them have a computed `border-radius` equal to `0px`

#### Scenario: Default theme
- **WHEN** a browser without a saved preference opens the dashboard
- **THEN** the theme and style defined in `default_theme`/`default_theme_style` (default Ocean Light, flat) are applied

#### Scenario: Contrast
- **WHEN** the contrast between text and background is measured on the theme colors
- **THEN** every text combination reaches at least 7:1

### Requirement: Windows XP theme with flat and 3D styles
The toolbar SHALL offer a "Theme" menu to choose among the registered themes (see "Theme registry with WCAG AAA contrast") and between the "Flat" and "No Flat" styles, indicating the active options. The Windows XP theme SHALL reproduce the Luna look (beige background, blue title bar, Tahoma font, tabs with an orange highlight on top, light buttons with a dark blue border and an orange highlight on hover), with darkened blues to keep white text at AAA. The "No Flat" style SHALL use gradients, bevels and shadows; the "Flat" style SHALL use the same colors without gradients or shadows. The choice MUST be remembered by the browser and applied before the first render. No theme or style MUST have rounded corners, and all of them MUST meet the contrast minimums of the theme registry.

#### Scenario: Switch to Windows XP No Flat
- **WHEN** the user chooses "Windows XP" and then "No Flat" in the "Theme" menu
- **THEN** the interface switches to the Luna look with gradients and the blue gradient toolbar, without reloading the page

#### Scenario: Flat style
- **WHEN** the user chooses "Flat" with the Windows XP theme active
- **THEN** the same colors are kept without gradients or shadows

#### Scenario: Preference persists
- **WHEN** the user reloads the page after choosing "Windows XP" and "No Flat"
- **THEN** the chosen theme and style remain applied

#### Scenario: No rounded corners in any theme
- **WHEN** a browser test goes through the visible elements in every theme and style combination
- **THEN** all of them have a computed `border-radius` equal to `0px`

#### Scenario: Storage unavailable
- **WHEN** `localStorage` is not available
- **THEN** the dashboard works with the default theme and a theme change lasts until the page is reloaded

### Requirement: Thin scrollbars
The dashboard scrollbars (outside the iframes) SHALL be thin and use the active theme colors, with the same look across the supported browsers: the standard properties `scrollbar-width: thin` and `scrollbar-color` (Chrome, Edge, Firefox) and 6 px `::-webkit-scrollbar` pseudo-elements only where `scrollbar-color` is not supported (Safari). Scrollbars inside the iframes belong to the embedded sites and are not changed.

#### Scenario: Thin themed scrollbar
- **WHEN** the home page is shown in the blue and black theme
- **THEN** the scrollable area uses `scrollbar-width: thin` with a black track and a `#6b7280` thumb

#### Scenario: Theme change
- **WHEN** the user switches to the Windows XP theme
- **THEN** the scrollbars use a `#ece9d8` track and a `#5a7aa0` thumb

### Requirement: Visual icon picker
The administration form SHALL offer, next to the icon field, a "Choose…" button that opens a modal with the Material Design Icons in a visual grid (icon + name). The modal SHALL show suggested icons when the search is empty, search among all icons of the installed font by English name or by accent-free synonym search terms, limit the grid to 240 results while reporting the total, mark the current icon, and offer "Use icon", "No icon" and "Cancel". The icon field SHALL show a preview of the chosen icon. The list of names MUST be generated from the installed icon package and loaded only when the modal opens.

#### Scenario: Pick an icon by search
- **WHEN** the administrator opens the picker, searches "server", chooses "server" and clicks "Use icon"
- **THEN** the modal closes, the field contains `mdi mdi-server` with the icon preview, and the value is saved with the item

#### Scenario: Current icon preselected
- **WHEN** the picker is opened for an item with the icon `mdi mdi-git`
- **THEN** the "git" icon appears marked and the footer preview shows `mdi mdi-git`

#### Scenario: Remove icon
- **WHEN** the administrator clicks "No icon"
- **THEN** the icon field becomes empty

#### Scenario: Too many results
- **WHEN** the search matches more than 240 icons
- **THEN** the grid shows 240 and reports "Showing 240 of N icons — refine the search"

### Requirement: Visible tab close button in every theme
The tab close button SHALL use each theme's own tokens (`tabCloseFg`, `tabCloseActiveFg`, `tabCloseHoverBg`, `tabCloseHoverFg`) with a minimum contrast of 4.5:1 against the tab: in the blue and black theme, `#a6b0bd` on the inactive tab and `#e6edf3` on the active tab; in Windows XP, `#3d3d3d`/black, with a red `#a11d23` highlight and a white "x" on hover. The click area SHALL be 22 px and the tab title MUST reserve space so it does not sit under the button.

#### Scenario: Active tab in dark theme
- **WHEN** an item tab is active in the blue and black theme
- **THEN** its close button is `#e6edf3` on the active tab surface

#### Scenario: Inactive tab in dark theme
- **WHEN** an item tab is inactive in the blue and black theme
- **THEN** its close button is `#a6b0bd` on the black background of the tab bar

### Requirement: Theme registry with WCAG AAA contrast
Themes SHALL be defined as data (color tokens, font, 3D style gradients and optional bevel and glow effects) in a single registry, and applied as CSS variables; component CSS MUST NOT contain theme-specific rules. The registry SHALL contain, grouped in the "Theme" menu: Dark themes (Blue and Black, Dark, Graphite, Midnight), Light themes (Light), Brand (Ocean Light, Ocean Dark, Bio Manager — the ad-manager Criare pattern), Inspired (Google, Microsoft, Super Mario Galaxy, Brasil, Winamp — palettes and typographic style only, without logos, sprites or other brand elements), Systems (Windows 10, Windows 7, Windows Vista, Windows XP, Windows 98, macOS, Classic Mac OS, Ubuntu, Linux KDE, xterm — palettes only, without logos) and Special (Matrix, High Contrast). Every text/background pair of every theme, including each stop of the 3D gradients, MUST have a minimum contrast of 7:1 (WCAG AAA); control icons 4.5:1; focus ring, scrollbar thumb and active tab highlight 3:1 — verified by an automated test for every registered theme. The Ocean themes MUST use the Ocean palette (navy `#1d3c55`, blue `#285783`, turquoise `#34cdd7` only as an accent, borders `#d6e3ea`) and the Inter font embedded in the binary.

#### Scenario: New theme is validated automatically
- **WHEN** a theme is added to the registry with a text contrast of 6.8:1
- **THEN** the contrast test fails reporting the pair, the usage and the measured ratio

#### Scenario: Grouped theme menu
- **WHEN** the user opens "Theme" in the toolbar
- **THEN** they see the groups Dark themes, Light themes, Brand, Inspired, Systems and Special, and the "Flat" and "No Flat" styles, with the active options highlighted in bold and with a side bar in the theme's accent color (without a check icon, keeping the labels aligned); the group of the active theme also appears in bold; menus and submenus open with a short transition (fade and slide, ~180 ms), the arrow of the open menu points up and the hover highlight is soft — with no animation when the system asks for reduced motion (`prefers-reduced-motion`)

#### Scenario: Retro bevels and Matrix glow
- **WHEN** the user chooses "Windows 98" or "Classic Mac OS" with "No Flat"
- **THEN** buttons and cards get a beveled relief; and in "Matrix" with 3D the titles and links get a green glow

#### Scenario: Light shadows on dark themes
- **WHEN** a dark theme is active, in any style
- **THEN** cards, dropdowns and the icon picker use a light-colored shadow (white by default, or the theme's own color, such as green in Matrix) instead of a black one; the light shadows are very subtle: cards in the Flat style with a 3% outline and a 6% opacity glow, in the No Flat style a 4% outline and a 9% glow, and dropdowns with 10%

#### Scenario: Toolbar icons stay visible
- **WHEN** any theme is active
- **THEN** the toolbar icons have the same color as the toolbar text

### Requirement: Slim bars and resolution scaling
The toolbar SHALL be 40 px and the tab bar 32 px high in Full HD, with a 13 px font. The interface SHALL scale proportionally to the usable CSS width: factor 1 up to 2299 px (Full HD), 1.25 from 2300 px (QHD) and 1.6 from 3400 px (4K at 100% system scaling), including the dimensions configured through JavaScript in the DHTMLX widgets (tab width/height, tree column). The page MUST NOT have horizontal scrolling at these resolutions and the administration form MUST limit its width on very wide screens.

#### Scenario: Full HD
- **WHEN** the dashboard is opened at 1920x1080
- **THEN** the toolbar is 40 px, the tabs 32 px and there is no horizontal scrolling

#### Scenario: QHD and 4K
- **WHEN** the dashboard is opened at 2560x1440 or 3840x2160
- **THEN** the toolbar and tabs are 40/32 px multiplied by 1.25 or 1.6, the tabs do not overlap and there is no horizontal scrolling

#### Scenario: Icon picker button aligned
- **WHEN** the item form is shown at any of these resolutions
- **THEN** the "Choose…" button sits on the same line as the icon field, aligned to the top and with the same height as the field box

### Requirement: Placement in menu and cards
The toolbar SHALL show only items with `placement` `menu` or `both`, and the Home tab only items with `card` or `both`, applying the rule to sub-items as well. The administration SHALL indicate in the tree the items shown only in the menu or only as a card.

#### Scenario: Menu-only item
- **WHEN** there is a top-level item with `placement="menu"`
- **THEN** it appears in the toolbar and does not appear as a card on Home

#### Scenario: Card-only item
- **WHEN** there is a top-level item with `placement="card"`
- **THEN** it appears as a card on Home, its links open normally, and it does not appear in the toolbar

### Requirement: Reorder home cards
Any analyst SHALL be able to move the Home tab cards directly, like Grafana panels, without login and without an arrange mode: by dragging the card by its header ("hand" cursor) and dropping it on another card. As an alternative without dragging (click, touch and keyboard), the header MUST have discreet "Move before/after" buttons, shown with the handle on hover or focus (always visible on screens without hover); the title remains a title and focus stays on the moved card. The card links MUST remain clickable. The order MUST be saved only in the analyst's browser (`localStorage`, protected by `try/catch`), MUST NOT be sent to the server or affect other analysts, and stays as the analyst left it (no reset button). The default order is the menu order defined in the Admin tree; cards missing from the saved order (added later) appear at the end and removed cards are ignored.

#### Scenario: Drag a card by its header
- **WHEN** the analyst drags the header of the last card onto the first one
- **THEN** the card becomes the first one and the order is saved in this browser

#### Scenario: Keyboard alternative
- **WHEN** the analyst clicks (or triggers via keyboard) "Move after" in a card header
- **THEN** the card swaps places with the next one, focus stays on the card and the order is saved in this browser

#### Scenario: Order is per browser
- **WHEN** the analyst reloads the page, or another person opens the dashboard in another browser
- **THEN** the analyst keeps seeing their order, the other person sees the default order and no change request is sent to the server

#### Scenario: Open dashboards follow admin changes
- **WHEN** an analyst already has the dashboard open and an administrator changes or reorders items in Admin
- **THEN** the analyst's dashboard applies the change without reloading the page when focus returns to the tab/window or on the periodic check (every 30 s), without closing open tabs; the check is paused while the tab is hidden, while a dropdown menu is open or during "Arrange cards", and `/api/menu` responds `304` (ETag) when nothing changed

### Requirement: Card icon column
The icon in each card header SHALL be aligned in the same column (same horizontal position and width) as the icons of the links listed below it, in every theme and scale. Cards without their own icon SHALL show a default folder icon to keep the alignment.

#### Scenario: Icons aligned
- **WHEN** the Home tab is shown
- **THEN** the icon of each card header and the icons of its links have the same horizontal coordinate (1 px tolerance) and the same width

### Requirement: Server default theme
The server SHALL expose in `GET /api/config` the default theme and style (`default_theme`, default `ocean-light`; `default_theme_style`, default `flat`), configurable via TOML, `DASHBOARD_DEFAULT_THEME`/`DASHBOARD_DEFAULT_THEME_STYLE` or `--default-theme`/`--default-theme-style`. An analyst without a choice saved in the browser MUST see that theme already on the first render; a choice made in the "Theme" menu MUST prevail over the default on that browser's following visits. Unknown theme names MUST fall back to the frontend default without error; values with an invalid format MUST be rejected at server startup.

#### Scenario: First visit
- **WHEN** an analyst without a saved preference opens the dashboard with the default configuration
- **THEN** the Ocean Light theme in the Flat style is applied

#### Scenario: Analyst choice wins
- **WHEN** the analyst chooses "Matrix" and "No Flat" and reloads the page
- **THEN** the Matrix 3D theme remains applied, regardless of `default_theme`

#### Scenario: Invalid configuration
- **WHEN** the server starts with `default_theme_style = "glossy"`
- **THEN** startup fails with a configuration error

