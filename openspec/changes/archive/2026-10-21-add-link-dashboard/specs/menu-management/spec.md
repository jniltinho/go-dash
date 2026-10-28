## ADDED Requirements

### Requirement: Hierarchical menu items
The system SHALL store menu items in SQLite in a hierarchical structure, where each item has a required `title`, optional `parent_id`, optional `url`, `target` (`iframe` or `blank`, default `iframe`), optional `icon`, optional `description`, an integer `position` and a boolean `enabled`. The depth of the hierarchy MUST NOT be limited by the data model.

#### Scenario: Create top-level menu
- **WHEN** an administrator creates an item with `title="Monitoring"` without `parent_id` and without `url`
- **THEN** the item is persisted as a top-level grouping menu

#### Scenario: Create nested submenu link
- **WHEN** an administrator creates an item with `title="Grafana"`, `url="https://grafana.example.com"`, `target="iframe"` and `parent_id` equal to the id of "Monitoring"
- **THEN** the item is persisted as a child of "Monitoring"

#### Scenario: Default target
- **WHEN** an item with `url` is created without a `target`
- **THEN** the item is persisted with `target="iframe"`

### Requirement: Menu item validation
The system SHALL reject invalid items with a descriptive error message. `title` MUST be non-empty after trimming and have at most 120 characters. `url`, when provided, MUST be absolute and use the `http` or `https` scheme. An item with `target="iframe"` MUST NOT have a URL with the same origin as the dashboard itself (compared with the configured `public_url` and with the request host). `target` MUST be `iframe` or `blank`. `parent_id`, when provided, MUST reference an existing item.

#### Scenario: Reject javascript URL
- **WHEN** an item is sent with `url="javascript:alert(1)"`
- **THEN** the system rejects the operation with a validation error and nothing is persisted

#### Scenario: Reject same-origin iframe
- **WHEN** the dashboard runs at `https://dash.example.com` and an item is sent with `url="https://dash.example.com/admin"` and `target="iframe"`
- **THEN** the system rejects the operation with a validation error on the `url` field

#### Scenario: Reject empty title
- **WHEN** an item is sent with `title="   "`
- **THEN** the system rejects the operation with a validation error

#### Scenario: Reject unknown parent
- **WHEN** an item is sent with a nonexistent `parent_id`
- **THEN** the system rejects the operation with a validation error

### Requirement: Prevent hierarchy cycles
The system SHALL prevent an item from being moved under itself or under any of its descendants.

#### Scenario: Move item under its descendant
- **WHEN** an administrator tries to set the `parent_id` of "Monitoring" to the id of its child "Grafana"
- **THEN** the system rejects the operation and the hierarchy remains unchanged

### Requirement: Ordering and reordering
The system SHALL return siblings ordered by ascending `position` and, on ties, by `id`. The system SHALL allow reordering and moving several items in a single atomic operation, validating the final tree resulting from the whole batch before persisting.

#### Scenario: Ordered siblings
- **WHEN** there are siblings with `position` 2, 0 and 1
- **THEN** the tree returns them in the order 0, 1, 2

#### Scenario: Cycle formed by combined moves
- **WHEN** a reorder batch moves A under B and, in the same batch, B under A
- **THEN** the whole batch is rejected, since the final tree would contain a cycle

#### Scenario: Atomic reorder failure
- **WHEN** a reorder operation contains one valid item and another that would create a cycle
- **THEN** no change is applied

### Requirement: Delete with cascade
When removing an item, the system SHALL also remove all of its descendants.

#### Scenario: Delete group
- **WHEN** an administrator removes "Monitoring", which has two children
- **THEN** "Monitoring" and its two children no longer exist

### Requirement: Public menu tree API
The system SHALL expose `GET /api/menu` returning, as JSON, the tree of enabled items. Disabled items MUST be omitted together with all of their descendants. The response MUST include for each node `id`, `title`, `url`, `target`, `icon`, `description` and `items` (children).

#### Scenario: Disabled branch hidden
- **WHEN** "Monitoring" has `enabled=false`
- **THEN** `GET /api/menu` returns neither "Monitoring" nor "Grafana"

#### Scenario: Empty database
- **WHEN** no item exists
- **THEN** `GET /api/menu` returns HTTP 200 with an empty list

### Requirement: Admin CRUD API
The system SHALL expose authenticated endpoints at `/api/admin/items` to list the complete tree (including disabled items), create, update, remove and reorder items. `PUT` MUST be a full replacement: omitted optional fields take `null` or the default value (`target=iframe`, `enabled=true`, `position=0`). Validation errors MUST return HTTP 400 with the JSON body `{"error": "...", "field": "..."}`; a nonexistent item MUST return HTTP 404; a body above 1 MiB MUST return HTTP 413.

#### Scenario: Update item
- **WHEN** an authenticated administrator sends `PUT /api/admin/items/{id}` with all fields and `target="blank"`
- **THEN** the item is updated, `updated_at` is refreshed and the response contains the updated item

#### Scenario: PUT omits optional field
- **WHEN** a `PUT` is sent for an item that had `icon` set, without the `icon` field
- **THEN** the item now has `icon=null`

#### Scenario: Unauthenticated access
- **WHEN** a request without a valid session calls `POST /api/admin/items`
- **THEN** the system responds HTTP 401 and nothing is persisted

### Requirement: Import and export
The system SHALL export the complete tree as a versioned JSON document `{"format":"dashboard-menu","version":1,"items":[...]}`, in which each item contains `title`, `url`, `target`, `icon`, `description`, `enabled`, `position` and `items` (children), without IDs. Import MUST accept the same format, generate new IDs, validate the whole document before writing and run in a single transaction. It MUST support the modes `append` (default; top-level items added at the end of the root) and `replace` (removes all existing items and imports). In the API the mode is the `?mode=append|replace` parameter; in the CLI, the `--replace` flag. An unknown `version` MUST be rejected.

#### Scenario: Round trip
- **WHEN** the tree is exported and imported in `replace` mode into an empty database
- **THEN** the resulting tree is equivalent to the original (titles, URLs, targets, icons, descriptions, `enabled`, order and hierarchy)

#### Scenario: Append import
- **WHEN** a document with 2 top-level items is imported in `append` mode into a database that already has 3 top-level items
- **THEN** the database now has 5 top-level items, with the imported ones after the existing ones, and the imported IDs are new

#### Scenario: Failed replace keeps existing data
- **WHEN** an invalid document is imported in `replace` mode over a database with items
- **THEN** the import is rejected and all existing items remain intact

#### Scenario: Unknown format version
- **WHEN** a document with `"version": 2` is imported
- **THEN** the import is rejected with an unsupported version error

#### Scenario: Invalid import
- **WHEN** the imported file contains an item with an invalid URL
- **THEN** the import is rejected entirely and the database remains unchanged

### Requirement: CLI menu administration
The binary SHALL provide the subcommands `menu list`, `menu add`, `menu update`, `menu remove`, `menu export` and `menu import`, which apply the same validations as the API.

#### Scenario: Add via CLI
- **WHEN** the user runs `dashboard menu add --title "Wiki" --url https://wiki.example.com --target blank`
- **THEN** the item is created and the generated id is printed to the output

#### Scenario: List as tree
- **WHEN** the user runs `dashboard menu list --tree`
- **THEN** the hierarchy is printed indented by level, showing id, title, target and URL

#### Scenario: Remove with children requires force
- **WHEN** the user runs `dashboard menu remove ID` on an item that has children without `--force`
- **THEN** the command fails with a non-zero exit code and nothing is removed

### Requirement: Item placement
Each item SHALL have the `placement` field with the values `menu` (only in the top menu), `card` (only as a card/link on the Home tab) or `both` (default). Items that existed before the field and import documents without the field MUST default to `both`. Invalid values MUST be rejected with a validation error on the `placement` field. The field SHALL be exported/imported, editable in the administration ("Show in") and in the CLI (`--placement`).

#### Scenario: Default placement
- **WHEN** an item is created without `placement`
- **THEN** the item is persisted with `placement="both"`

#### Scenario: Invalid placement
- **WHEN** an item is sent with `placement="sidebar"`
- **THEN** the system rejects the operation with a validation error on the `placement` field

#### Scenario: Old export
- **WHEN** an export document without `placement` is imported
- **THEN** the imported items have `placement="both"`

### Requirement: Duplicate an item and copy its URL
The administration SHALL offer "Duplicate" in the toolbar: with an item selected, the form switches to a new, not yet saved item with the same URL, target, placement, icon, description and state, under the same parent, positioned right after the original and with the title suffixed with " (copy)" (respecting the 120-character limit); the sub-items MUST NOT be copied and nothing is written until "Save". The URL field SHALL have a "Copy" button, which copies the URL to the clipboard (with a fallback when the clipboard API is not available, such as over HTTP without TLS).

#### Scenario: Duplicate then save
- **WHEN** the admin selects "Gitea", clicks "Duplicate" and then "Save"
- **THEN** "Gitea (copy)" appears with the same URL, in the same group, right after the original

#### Scenario: Duplicate without selection
- **WHEN** the admin clicks "Duplicate" with no item selected
- **THEN** they see the message "Select an item to duplicate" and nothing changes

#### Scenario: Copy URL
- **WHEN** the admin clicks "Copy" next to the URL
- **THEN** the URL goes to the clipboard and "URL copied" appears
