import { ref } from 'vue'

import { storageKey } from './basePath'
import type { ToolbarElement } from './menu'

/**
 * UI translations. English is the default; every other dictionary must have
 * exactly the same keys (enforced by its type and by a unit test). Messages
 * use `{name}` placeholders filled by `t(key, params)` — plain text only,
 * never HTML.
 */
export type Locale = 'en' | 'pt-BR'

/** Locales offered in the language menu, labelled in their own language. */
export const LOCALES: ReadonlyArray<{ locale: Locale; label: string }> = [
  { locale: 'en', label: 'English' },
  { locale: 'pt-BR', label: 'Português (Brasil)' },
]

export const DEFAULT_LOCALE: Locale = 'en'

const en = {
  // Shared
  'common.cancel': 'Cancel',
  'common.close': 'Close',
  'common.delete': 'Delete',
  'common.deleteConfirm': 'Delete "{title}"?',
  'common.edit': 'Edit',
  'common.save': 'Save',
  'common.loading': 'Loading…',
  'common.retry': 'Try again',
  'common.password': 'Password',
  'common.unexpectedError': 'Unexpected error',
  'common.backToDashboard': 'Back to the dashboard',
  'common.tooManyAttempts': 'Too many attempts. Wait a minute and try again.',
  'common.language': 'Language',

  // Toolbar
  'toolbar.theme': 'Theme',
  'toolbar.lock': 'Lock',
  'toolbar.lockTooltip': 'Lock this dashboard (asks for the password again)',
  'toolbar.open': 'Open {title}',
  'themeGroup.dark': 'Dark',
  'themeGroup.light': 'Light',
  'themeGroup.brand': 'Brand',
  'themeGroup.inspired': 'Inspired',
  'themeGroup.system': 'Systems',
  'themeGroup.special': 'Special',
  'menu.copySuffix': 'copy',

  // Dashboard
  'dashboard.tabLimit': 'Limit of {limit} tabs: "{title}" was closed',
  'dashboard.itemNotFound': 'Item not found',
  'dashboard.openInNewTab': 'Open in new tab',
  'dashboard.reload': 'Reload',
  'dashboard.sameOrigin': 'Open this item in a new tab',
  'dashboard.notFound': 'Dashboard not found',
  'dashboard.notFoundBefore': 'The address',
  'dashboard.notFoundAfter': 'does not match any dashboard.',
  'dashboard.goHome': 'Go to Home',
  'dashboard.errorTitle': 'Error',
  'dashboard.loadFailed': 'Could not load the dashboard.',

  // Home cards
  'home.menuLoadFailed': 'Could not load the menu',
  'home.empty': 'No cards to show. Add links shown as cards in the',
  'home.dragHint': 'Drag to move the card',
  'home.moveBefore': 'Move {title} before',
  'home.moveBeforeShort': 'Move before',
  'home.moveAfter': 'Move {title} after',
  'home.moveAfterShort': 'Move after',
  'home.newTab': 'New tab',

  // Unlock and login
  'unlock.protected': 'This dashboard is password protected.',
  'unlock.wrongPassword': 'Wrong password.',
  'unlock.failed': 'Could not unlock. Try again.',
  'unlock.submit': 'Unlock',
  'login.invalid': 'Invalid password.',
  'login.submit': 'Sign in',
  'adminDisabled.title': 'Web admin disabled',
  'adminDisabled.intro':
    'No administrator password has been configured. To enable it, use one of these options:',
  'adminDisabled.envVar': 'environment variable',
  'adminDisabled.bcryptHash': '(bcrypt hash)',
  'adminDisabled.cli': 'Menus can also be managed from the CLI:',

  // Icon picker
  'icons.title': 'Choose icon',
  'icons.placeholder': 'Search icons (e.g. chart, server, home, gráfico, servidor)…',
  'icons.searchLabel': 'Search icons',
  'icons.suggested': 'Suggested — type to search all icons.',
  'icons.loading': 'Loading icons…',
  'icons.none': 'No icons found for "{query}".',
  'icons.truncated': 'Showing {limit} of {total} icons — refine the search.',
  'icons.found': '{total} icon(s) found.',
  'icons.gridLabel': 'Icons (use the arrow keys to navigate and Enter to choose)',
  'icons.noneSelected': 'No icon selected',
  'icons.noIcon': 'No icon',
  'icons.use': 'Use icon',

  // Dashboards panel
  'dashboards.slugInvalid': 'Use 1 to 40 lowercase letters, digits or hyphens.',
  'dashboards.slugReserved': '"{slug}" is reserved.',
  'dashboards.slugAddress': 'Address: {path}',
  'dashboards.configTitle': '{title} (configured title)',
  'dashboards.saved': 'Dashboard saved',
  'dashboards.created': 'Dashboard created',
  'dashboards.deleteHeader': 'Delete dashboard',
  'dashboards.deleteWithItems': 'Delete "{title}" and its {count} items? Anyone using {path} loses access.',
  'dashboards.deleted': 'Dashboard deleted',
  'dashboards.passwordMismatch': 'Passwords do not match.',
  'dashboards.passwordSet': 'Password set for {title}; visitors must sign in again',
  'dashboards.removePassword': 'Remove password',
  'dashboards.removePasswordText': '"{title}" will be public at {path}. Continue?',
  'dashboards.passwordRemoved': 'Password removed',
  'dashboards.colTitle': 'Title',
  'dashboards.colAddress': 'Address',
  'dashboards.colItems': 'Items',
  'dashboards.colAccess': 'Access',
  'dashboards.colActions': 'Actions',
  'dashboards.protected': 'Protected',
  'dashboards.public': 'Public',
  'dashboards.setPassword': 'Set password',
  'dashboards.passwordFor': 'Password for {title} ({path})',
  'dashboards.newPassword': 'New password (8 to 72 characters)',
  'dashboards.repeatPassword': 'Repeat the password',
  'dashboards.savePassword': 'Save password',
  'dashboards.editing': 'Editing {title}',
  'dashboards.new': 'New dashboard',
  'dashboards.titleLabel': 'Title (shown in the toolbar and the browser tab)',
  'dashboards.titlePlaceholder': 'e.g. Acme',
  'dashboards.slugLabel': 'Address (slug)',
  'dashboards.slugDefault': '/ (default dashboard)',
  'dashboards.slugPlaceholder': 'e.g. acme',
  'dashboards.cancelEdit': 'Cancel editing',
  'dashboards.create': 'Create dashboard',

  // Admin
  'admin.menuOnly': 'menu only',
  'admin.cardOnly': 'card only',
  'admin.disabled': 'disabled',
  'admin.newItemRoot': 'New item at the top level',
  'admin.newItemIn': 'New item inside "{parent}"',
  'admin.editing': 'Editing #{id}',
  'admin.editingIn': 'Editing #{id} (inside "{parent}")',
  'admin.selectToDuplicate': 'Select an item to duplicate',
  'admin.copyOf': 'Copy of "{title}" — adjust it and click Save',
  'admin.copyOfIn': 'Copy of "{title}" inside "{parent}" — adjust it and click Save',
  'admin.noUrl': 'No URL to copy',
  'admin.copyFailed': 'Could not copy the URL',
  'admin.urlCopied': 'URL copied',
  'admin.itemSaved': 'Item saved',
  'admin.deleteItem': 'Delete item',
  'admin.deleteWithChildren': 'Delete "{title}" and its {count} subitems?',
  'admin.itemDeleted': 'Item deleted',
  'admin.orderSaved': 'Order saved',
  'admin.importTitle': 'Import menu into {dashboard}',
  'admin.importHelp': 'Paste the exported JSON (dashboard-menu v1 format).',
  'admin.importMode': 'Mode',
  'admin.importAppend': 'Append to the end',
  'admin.importReplace': 'Replace everything in this dashboard',
  'admin.import': 'Import',
  'admin.export': 'Export',
  'admin.invalidJson': 'Invalid JSON',
  'admin.imported': '{count} items imported',
  'admin.manageDashboards': 'Manage dashboards…',
  'admin.new': 'New',
  'admin.newChild': 'New subitem',
  'admin.duplicate': 'Duplicate',
  'admin.viewDashboard': 'Dashboard',
  'admin.logout': 'Sign out',
  'admin.treeHeader': 'Menu items',
  'admin.formHeader': 'Details',
  'admin.fieldTitle': 'Title',
  'admin.fieldUrl': 'URL (empty = group of submenus only)',
  'admin.copy': 'Copy',
  'admin.fieldTarget': 'Open in',
  'admin.targetIframe': 'Dashboard tab (iframe)',
  'admin.targetBlank': 'New browser tab',
  'admin.fieldPlacement': 'Show in',
  'admin.placementBoth': 'Top menu and Home card',
  'admin.placementMenu': 'Top menu only',
  'admin.placementCard': 'Home card only',
  'admin.fieldIcon': 'Icon (MDI class)',
  'admin.pickIcon': 'Choose…',
  'admin.fieldDescription': 'Description (tooltip)',
  'admin.fieldPosition': 'Position',
  'admin.fieldEnabled': 'Enabled',

  // API errors (see messages.ts)
  'error.titleRequired': 'Title is required.',
  'error.titleTooLong': 'Title must have at most {max} characters.',
  'error.target': 'Choose "Dashboard tab" or "New browser tab".',
  'error.placement': 'Choose where to show it: top menu, card or both.',
  'error.invalidUrl': 'Invalid URL.',
  'error.urlScheme': 'The URL must start with http:// or https://.',
  'error.urlAbsolute': 'Enter the full URL, for example https://example.com.',
  'error.sameOrigin': "The dashboard's own addresses can only open in a new browser tab.",
  'error.parentMissing': 'Parent item #{id} does not exist.',
  'error.cycle': 'An item cannot be placed inside itself or one of its subitems.',
  'error.itemMissing': 'Item #{id} does not exist.',
  'error.hasChildren': 'The item has {count} subitems.',
  'error.format': 'The file is not a dashboard export (unknown format).',
  'error.version': 'Unsupported export file version.',
  'error.importMode': 'Invalid import mode.',
  'error.nullItem': 'The file contains an empty item.',
  'error.itemNotFound': 'Item not found.',
  'error.authRequired': 'Session expired. Please sign in again.',
  'error.csrf': 'Invalid session. Reload the page.',
  'error.tooManyAttempts': 'Too many attempts. Wait a minute.',
  'error.invalidCredentials': 'Invalid password.',
  'error.invalidRequest': 'Invalid request.',
  'error.tooLarge': 'Content too large (1 MiB limit).',
  'error.adminDisabled': 'The web admin interface is disabled.',
  'error.crossOrigin': 'Cross-origin request blocked.',
  'error.invalidId': 'Invalid identifier.',
  'error.dashboardLocked': 'Dashboard is password protected.',
  'error.dashboardNotFound': 'Dashboard not found.',
  'error.notProtected': 'This dashboard has no password.',
  'error.dashboardHasItems': 'The dashboard has items. Confirm to delete them too.',
  'error.wrongPassword': 'Wrong password.',
  'error.passwordLength': 'The password must have {min} to {max} characters.',
  'error.slugRequired': 'Enter the address (slug).',
  'error.slugInvalid':
    'Invalid address: use 1 to 40 lowercase letters, digits or hyphens (no hyphen at either end).',
  'error.slugReserved': 'The address "{slug}" is reserved.',
  'error.slugInUse': 'The address "{slug}" is already in use.',
  'error.defaultNoSlug': 'The default dashboard has no address of its own (it lives at /).',
  'error.defaultUndeletable': 'The default dashboard cannot be deleted.',
  'error.invalidDashboardId': 'Invalid dashboard identifier.',
  'error.notFound': 'Resource not found.',
  'error.internal': 'Internal server error. Please try again.',
  'error.unknown': 'The operation could not be completed ({message}).',
} as const

export type MessageKey = keyof typeof en

const ptBR: Record<MessageKey, string> = {
  'common.cancel': 'Cancelar',
  'common.close': 'Fechar',
  'common.delete': 'Excluir',
  'common.deleteConfirm': 'Excluir "{title}"?',
  'common.edit': 'Editar',
  'common.save': 'Salvar',
  'common.loading': 'Carregando…',
  'common.retry': 'Tentar novamente',
  'common.password': 'Senha',
  'common.unexpectedError': 'Erro inesperado',
  'common.backToDashboard': 'Voltar ao painel',
  'common.tooManyAttempts': 'Muitas tentativas. Aguarde um minuto e tente novamente.',
  'common.language': 'Idioma',

  'toolbar.theme': 'Tema',
  'toolbar.lock': 'Bloquear',
  'toolbar.lockTooltip': 'Bloquear este dashboard (pede a senha de novo)',
  'toolbar.open': 'Abrir {title}',
  'themeGroup.dark': 'Escuros',
  'themeGroup.light': 'Claros',
  'themeGroup.brand': 'Marca',
  'themeGroup.inspired': 'Inspirados',
  'themeGroup.system': 'Sistemas',
  'themeGroup.special': 'Especiais',
  'menu.copySuffix': 'cópia',

  'dashboard.tabLimit': 'Limite de {limit} abas: "{title}" foi fechada',
  'dashboard.itemNotFound': 'Item não encontrado',
  'dashboard.openInNewTab': 'Abrir em nova aba',
  'dashboard.reload': 'Recarregar',
  'dashboard.sameOrigin': 'Abra este item em nova aba',
  'dashboard.notFound': 'Dashboard não encontrado',
  'dashboard.notFoundBefore': 'O endereço',
  'dashboard.notFoundAfter': 'não corresponde a nenhum dashboard.',
  'dashboard.goHome': 'Ir para Home',
  'dashboard.errorTitle': 'Erro',
  'dashboard.loadFailed': 'Não foi possível carregar o dashboard.',

  'home.menuLoadFailed': 'Não foi possível carregar o menu',
  'home.empty': 'Nenhum card para exibir. Cadastre links exibidos como card no',
  'home.dragHint': 'Arraste para mover o card',
  'home.moveBefore': 'Mover {title} para antes',
  'home.moveBeforeShort': 'Mover para antes',
  'home.moveAfter': 'Mover {title} para depois',
  'home.moveAfterShort': 'Mover para depois',
  'home.newTab': 'Nova aba',

  'unlock.protected': 'Este dashboard é protegido por senha.',
  'unlock.wrongPassword': 'Senha incorreta.',
  'unlock.failed': 'Não foi possível desbloquear. Tente novamente.',
  'unlock.submit': 'Entrar',
  'login.invalid': 'Senha inválida.',
  'login.submit': 'Entrar',
  'adminDisabled.title': 'Admin web desabilitado',
  'adminDisabled.intro':
    'Nenhuma senha de administrador foi configurada. Para habilitar, use uma das opções:',
  'adminDisabled.envVar': 'variável',
  'adminDisabled.bcryptHash': '(hash bcrypt)',
  'adminDisabled.cli': 'Os menus também podem ser gerenciados pela CLI:',

  'icons.title': 'Escolher ícone',
  'icons.placeholder': 'Buscar ícone (ex.: gráfico, servidor, casa, chart, database)…',
  'icons.searchLabel': 'Buscar ícone',
  'icons.suggested': 'Sugeridos — digite para buscar entre todos os ícones.',
  'icons.loading': 'Carregando ícones…',
  'icons.none': 'Nenhum ícone encontrado para "{query}".',
  'icons.truncated': 'Mostrando {limit} de {total} ícones — refine a busca.',
  'icons.found': '{total} ícone(s) encontrado(s).',
  'icons.gridLabel': 'Ícones (use as setas para navegar e Enter para usar)',
  'icons.noneSelected': 'Nenhum ícone selecionado',
  'icons.noIcon': 'Sem ícone',
  'icons.use': 'Usar ícone',

  'dashboards.slugInvalid': 'Use de 1 a 40 letras minúsculas, números ou hífens.',
  'dashboards.slugReserved': '"{slug}" é reservado.',
  'dashboards.slugAddress': 'Endereço: {path}',
  'dashboards.configTitle': '{title} (título da configuração)',
  'dashboards.saved': 'Dashboard salvo',
  'dashboards.created': 'Dashboard criado',
  'dashboards.deleteHeader': 'Excluir dashboard',
  'dashboards.deleteWithItems':
    'Excluir "{title}" e seus {count} itens? Quem estiver usando {path} perde o acesso.',
  'dashboards.deleted': 'Dashboard excluído',
  'dashboards.passwordMismatch': 'As senhas não conferem.',
  'dashboards.passwordSet': 'Senha definida para {title}; visitantes precisam entrar de novo',
  'dashboards.removePassword': 'Remover senha',
  'dashboards.removePasswordText': '"{title}" ficará público em {path}. Continuar?',
  'dashboards.passwordRemoved': 'Senha removida',
  'dashboards.colTitle': 'Título',
  'dashboards.colAddress': 'Endereço',
  'dashboards.colItems': 'Itens',
  'dashboards.colAccess': 'Acesso',
  'dashboards.colActions': 'Ações',
  'dashboards.protected': 'Com senha',
  'dashboards.public': 'Público',
  'dashboards.setPassword': 'Definir senha',
  'dashboards.passwordFor': 'Senha de {title} ({path})',
  'dashboards.newPassword': 'Nova senha (8 a 72 caracteres)',
  'dashboards.repeatPassword': 'Repita a senha',
  'dashboards.savePassword': 'Salvar senha',
  'dashboards.editing': 'Editando {title}',
  'dashboards.new': 'Novo dashboard',
  'dashboards.titleLabel': 'Título (aparece na barra e na aba do navegador)',
  'dashboards.titlePlaceholder': 'Ex.: Acme',
  'dashboards.slugLabel': 'Endereço (slug)',
  'dashboards.slugDefault': '/ (dashboard padrão)',
  'dashboards.slugPlaceholder': 'ex.: acme',
  'dashboards.cancelEdit': 'Cancelar edição',
  'dashboards.create': 'Criar dashboard',

  'admin.menuOnly': 'só menu',
  'admin.cardOnly': 'só card',
  'admin.disabled': 'desativado',
  'admin.newItemRoot': 'Novo item no primeiro nível',
  'admin.newItemIn': 'Novo item dentro de "{parent}"',
  'admin.editing': 'Editando #{id}',
  'admin.editingIn': 'Editando #{id} (dentro de "{parent}")',
  'admin.selectToDuplicate': 'Selecione um item para duplicar',
  'admin.copyOf': 'Cópia de "{title}" — ajuste e clique em Salvar',
  'admin.copyOfIn': 'Cópia de "{title}" dentro de "{parent}" — ajuste e clique em Salvar',
  'admin.noUrl': 'Sem URL para copiar',
  'admin.copyFailed': 'Não foi possível copiar a URL',
  'admin.urlCopied': 'URL copiada',
  'admin.itemSaved': 'Item salvo',
  'admin.deleteItem': 'Excluir item',
  'admin.deleteWithChildren': 'Excluir "{title}" e seus {count} subitens?',
  'admin.itemDeleted': 'Item excluído',
  'admin.orderSaved': 'Ordem salva',
  'admin.importTitle': 'Importar menu em {dashboard}',
  'admin.importHelp': 'Cole o JSON exportado (formato dashboard-menu v1).',
  'admin.importMode': 'Modo',
  'admin.importAppend': 'Adicionar ao final',
  'admin.importReplace': 'Substituir tudo deste dashboard',
  'admin.import': 'Importar',
  'admin.export': 'Exportar',
  'admin.invalidJson': 'JSON inválido',
  'admin.imported': '{count} itens importados',
  'admin.manageDashboards': 'Gerenciar dashboards…',
  'admin.new': 'Novo',
  'admin.newChild': 'Novo subitem',
  'admin.duplicate': 'Duplicar',
  'admin.viewDashboard': 'Painel',
  'admin.logout': 'Sair',
  'admin.treeHeader': 'Itens do menu',
  'admin.formHeader': 'Detalhes',
  'admin.fieldTitle': 'Título',
  'admin.fieldUrl': 'URL (vazio = apenas agrupa submenus)',
  'admin.copy': 'Copiar',
  'admin.fieldTarget': 'Abrir em',
  'admin.targetIframe': 'Aba do painel (iframe)',
  'admin.targetBlank': 'Nova aba do navegador',
  'admin.fieldPlacement': 'Exibir em',
  'admin.placementBoth': 'Menu superior e card na Home',
  'admin.placementMenu': 'Somente no menu superior',
  'admin.placementCard': 'Somente como card na Home',
  'admin.fieldIcon': 'Ícone (classe MDI)',
  'admin.pickIcon': 'Escolher…',
  'admin.fieldDescription': 'Descrição (tooltip)',
  'admin.fieldPosition': 'Posição',
  'admin.fieldEnabled': 'Ativo',

  'error.titleRequired': 'O título é obrigatório.',
  'error.titleTooLong': 'O título deve ter no máximo {max} caracteres.',
  'error.target': 'Escolha "Aba do painel" ou "Nova aba do navegador".',
  'error.placement': 'Escolha onde exibir: menu superior, card ou ambos.',
  'error.invalidUrl': 'URL inválida.',
  'error.urlScheme': 'A URL deve começar com http:// ou https://.',
  'error.urlAbsolute': 'Informe a URL completa, por exemplo https://exemplo.com.',
  'error.sameOrigin': 'Endereços do próprio painel só podem abrir em nova aba do navegador.',
  'error.parentMissing': 'O item pai #{id} não existe.',
  'error.cycle': 'Um item não pode ficar dentro de si mesmo ou de um subitem.',
  'error.itemMissing': 'O item #{id} não existe.',
  'error.hasChildren': 'O item possui {count} subitens.',
  'error.format': 'Arquivo não é uma exportação do painel (formato desconhecido).',
  'error.version': 'Versão do arquivo de exportação não suportada.',
  'error.importMode': 'Modo de importação inválido.',
  'error.nullItem': 'O arquivo contém um item vazio.',
  'error.itemNotFound': 'Item não encontrado.',
  'error.authRequired': 'Sessão expirada. Entre novamente.',
  'error.csrf': 'Sessão inválida. Recarregue a página.',
  'error.tooManyAttempts': 'Muitas tentativas. Aguarde um minuto.',
  'error.invalidCredentials': 'Senha inválida.',
  'error.invalidRequest': 'Requisição inválida.',
  'error.tooLarge': 'Conteúdo grande demais (limite de 1 MiB).',
  'error.adminDisabled': 'A administração web está desabilitada.',
  'error.crossOrigin': 'Requisição de outra origem bloqueada.',
  'error.invalidId': 'Identificador inválido.',
  'error.dashboardLocked': 'Dashboard protegido por senha.',
  'error.dashboardNotFound': 'Dashboard não encontrado.',
  'error.notProtected': 'Este dashboard não tem senha.',
  'error.dashboardHasItems': 'O dashboard possui itens. Confirme para excluí-los também.',
  'error.wrongPassword': 'Senha incorreta.',
  'error.passwordLength': 'A senha deve ter de {min} a {max} caracteres.',
  'error.slugRequired': 'Informe o endereço (slug).',
  'error.slugInvalid':
    'Endereço inválido: use de 1 a 40 letras minúsculas, números ou hífens (sem hífen nas pontas).',
  'error.slugReserved': 'O endereço "{slug}" é reservado.',
  'error.slugInUse': 'O endereço "{slug}" já está em uso.',
  'error.defaultNoSlug': 'O dashboard padrão não tem endereço próprio (fica em /).',
  'error.defaultUndeletable': 'O dashboard padrão não pode ser excluído.',
  'error.invalidDashboardId': 'Identificador de dashboard inválido.',
  'error.notFound': 'Recurso não encontrado.',
  'error.internal': 'Erro interno no servidor. Tente novamente.',
  'error.unknown': 'Não foi possível concluir a operação ({message}).',
}

export const DICTIONARIES: Readonly<Record<Locale, Readonly<Record<MessageKey, string>>>> = {
  en,
  'pt-BR': ptBR,
}

export type MessageParams = Readonly<Record<string, string | number>>

/** Fills `{name}` placeholders; unknown placeholders are left as they are. */
export function interpolate(text: string, params?: MessageParams): string {
  if (!params) return text
  return text.replace(/\{(\w+)\}/g, (whole, name: string) =>
    Object.hasOwn(params, name) ? String(params[name]) : whole,
  )
}

/** Message in a given locale (pure; falls back to English). */
export function translate(locale: Locale, key: MessageKey, params?: MessageParams): string {
  return interpolate(DICTIONARIES[locale][key] ?? en[key], params)
}

/**
 * Current UI locale. Reactive: templates and computed values that call `t()`
 * re-render when it changes; DHTMLX widgets are rebuilt by their views.
 */
export const currentLocale = ref<Locale>(DEFAULT_LOCALE)

/** Message in the current locale. */
export function t(key: MessageKey, params?: MessageParams): string {
  return translate(currentLocale.value, key, params)
}

export function isLocale(value: unknown): value is Locale {
  return LOCALES.some((l) => l.locale === value)
}

/** Saved choice, else a Portuguese browser language, else English. */
export function resolveLocale(saved: unknown, browserLanguage: string | undefined): Locale {
  if (isLocale(saved)) return saved
  if (browserLanguage && /^pt\b/i.test(browserLanguage)) return 'pt-BR'
  return DEFAULT_LOCALE
}

const STORAGE_NAME = 'dashboard.locale'

/** The viewer's saved choice, or null; storage may be unavailable. */
export function loadSavedLocale(): Locale | null {
  try {
    const raw = localStorage.getItem(storageKey(STORAGE_NAME))
    return isLocale(raw) ? raw : null
  } catch {
    return null
  }
}

export function saveLocale(locale: Locale): void {
  try {
    localStorage.setItem(storageKey(STORAGE_NAME), locale)
  } catch {
    // Preference is a convenience only.
  }
}

/** Initial locale for this viewer (saved choice, then the browser language). */
export function initialLocale(): Locale {
  return resolveLocale(loadSavedLocale(), typeof navigator === 'undefined' ? undefined : navigator.language)
}

/** Makes `locale` current and sets `<html lang>`. */
export function applyLocale(locale: Locale, root: HTMLElement = document.documentElement): void {
  currentLocale.value = locale
  root.lang = locale
}

/** Toolbar dropdown to pick the UI language; the active one is highlighted. */
export function localeMenu(current: Locale = currentLocale.value): ToolbarElement {
  return {
    id: 'lang',
    type: 'navItem',
    value: t('common.language'),
    icon: 'mdi mdi-translate',
    items: LOCALES.map(({ locale, label }) => ({
      id: `lang-${locale}`,
      type: 'menuItem',
      value: label,
      ...(locale === current ? { css: 'menu-current' } : {}),
    })),
  }
}

/** Parses a language menu click (`lang-<locale>`), or null for other items. */
export function parseLocaleId(id: string | number): Locale | null {
  const value = String(id)
  if (!value.startsWith('lang-')) return null
  const locale = value.slice('lang-'.length)
  return isLocale(locale) ? locale : null
}
