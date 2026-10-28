/** Icon helpers for the Material Design Icons font (`mdi mdi-<name>`). */

/** Icons offered first, chosen for a link dashboard. */
export const SUGGESTED_ICONS: readonly string[] = [
  'view-dashboard',
  'home',
  'link-variant',
  'web',
  'earth',
  'chart-line',
  'chart-bar',
  'chart-pie',
  'monitor-dashboard',
  'gauge',
  'pulse',
  'bell',
  'alert',
  'server',
  'database',
  'cloud',
  'lan',
  'router-wireless',
  'shield-lock',
  'key',
  'account',
  'account-group',
  'email',
  'chat',
  'phone',
  'calendar',
  'clock-outline',
  'file-document',
  'folder',
  'book-open-variant',
  'wikipedia',
  'git',
  'github',
  'gitlab',
  'code-braces',
  'console',
  'docker',
  'kubernetes',
  'bug',
  'cog',
  'tools',
  'help-circle',
  'map',
  'map-marker',
  'cart',
  'cash',
  'hospital-building',
  'medical-bag',
  'pill',
  'truck',
  'factory',
  'school',
]

/**
 * English search terms that icon names do not contain, mapped to icon name
 * fragments. Keys are accent-free and lower case.
 */
const EN_SYNONYMS: Record<string, string[]> = {
  house: ['home'],
  dashboard: ['view-dashboard', 'monitor-dashboard'],
  graph: ['chart'],
  report: ['chart', 'file-chart'],
  settings: ['cog'],
  preferences: ['cog', 'tune'],
  user: ['account'],
  users: ['account-group', 'account-multiple'],
  people: ['account-group', 'account-multiple'],
  team: ['account-group'],
  person: ['account', 'human'],
  password: ['key', 'lock'],
  security: ['shield', 'lock'],
  mail: ['email'],
  network: ['lan', 'router'],
  search: ['magnify'],
  find: ['magnify'],
  trash: ['delete'],
  bin: ['delete'],
  remove: ['delete', 'minus'],
  edit: ['pencil'],
  add: ['plus'],
  money: ['cash', 'currency'],
  finance: ['cash', 'bank'],
  health: ['medical', 'heart-pulse', 'hospital'],
  medicine: ['pill', 'medication'],
  delivery: ['truck'],
  shipping: ['truck'],
  shop: ['cart', 'store'],
  shopping: ['cart'],
  time: ['clock', 'timer'],
  notification: ['bell'],
  warning: ['alert'],
  location: ['map-marker'],
  place: ['map-marker'],
  photo: ['image', 'camera'],
  picture: ['image'],
  support: ['face-agent', 'lifebuoy', 'help'],
  docs: ['file-document', 'book'],
  documentation: ['file-document', 'book'],
  wiki: ['wikipedia', 'book'],
  terminal: ['console'],
  shell: ['console'],
  internet: ['web', 'earth'],
  website: ['web'],
  site: ['web'],
  globe: ['earth', 'web'],
  spreadsheet: ['table', 'file-excel'],
  favorite: ['star', 'bookmark'],
  exit: ['logout'],
  signin: ['login'],
}

/**
 * Portuguese search terms mapped to English icon name fragments. Both maps are
 * always searched, whatever the UI language. Keys are accent-free and lower case.
 */
const PT_SYNONYMS: Record<string, string[]> = {
  casa: ['home', 'house'],
  inicio: ['home'],
  painel: ['view-dashboard', 'monitor-dashboard'],
  grafico: ['chart'],
  graficos: ['chart'],
  relatorio: ['chart', 'file-chart', 'report'],
  banco: ['database', 'bank'],
  dados: ['database', 'data'],
  servidor: ['server'],
  nuvem: ['cloud'],
  rede: ['lan', 'network', 'router'],
  seguranca: ['shield', 'lock', 'security'],
  senha: ['key', 'lock', 'password'],
  chave: ['key'],
  usuario: ['account'],
  usuarios: ['account-group', 'account-multiple'],
  pessoa: ['account', 'human'],
  equipe: ['account-group'],
  correio: ['email', 'mail'],
  mensagem: ['message', 'chat'],
  telefone: ['phone'],
  calendario: ['calendar'],
  agenda: ['calendar'],
  relogio: ['clock'],
  tempo: ['clock', 'timer', 'weather'],
  arquivo: ['file'],
  documento: ['file-document'],
  pasta: ['folder'],
  livro: ['book'],
  ajuda: ['help'],
  configuracao: ['cog', 'settings'],
  configuracoes: ['cog', 'settings'],
  ferramenta: ['tools', 'wrench'],
  ferramentas: ['tools', 'wrench'],
  codigo: ['code'],
  terminal: ['console'],
  alerta: ['alert', 'bell'],
  aviso: ['alert', 'bell'],
  notificacao: ['bell'],
  monitor: ['monitor'],
  monitoramento: ['monitor-dashboard', 'gauge', 'pulse'],
  mapa: ['map'],
  local: ['map-marker'],
  carrinho: ['cart'],
  compra: ['cart', 'shopping'],
  dinheiro: ['cash', 'currency'],
  financeiro: ['cash', 'finance', 'bank'],
  hospital: ['hospital'],
  saude: ['medical', 'heart-pulse', 'hospital'],
  remedio: ['pill', 'medication'],
  medicamento: ['pill', 'medication'],
  caminhao: ['truck'],
  entrega: ['truck'],
  fabrica: ['factory'],
  escola: ['school'],
  busca: ['magnify'],
  pesquisa: ['magnify'],
  impressora: ['printer'],
  video: ['video'],
  camera: ['camera'],
  foto: ['image', 'camera'],
  imagem: ['image'],
  musica: ['music'],
  estrela: ['star'],
  coracao: ['heart'],
  lixeira: ['delete', 'trash'],
  editar: ['pencil'],
  link: ['link'],
  pizza: ['pie', 'pizza'],
  barra: ['bar'],
  barras: ['bar'],
  linha: ['line'],
  area: ['area'],
  seta: ['arrow'],
  mais: ['plus'],
  adicionar: ['plus', 'add'],
  fechar: ['close'],
  olho: ['eye'],
  cadeado: ['lock'],
  globo: ['earth', 'web'],
  internet: ['web', 'earth'],
  site: ['web'],
  favorito: ['star', 'bookmark'],
  marcador: ['bookmark'],
  download: ['download'],
  baixar: ['download'],
  enviar: ['send', 'upload'],
  upload: ['upload'],
  sair: ['logout', 'exit'],
  entrar: ['login'],
  lista: ['list', 'format-list'],
  tabela: ['table'],
  planilha: ['table', 'spreadsheet', 'file-excel'],
  nota: ['note'],
  ticket: ['ticket'],
  chamado: ['ticket', 'face-agent'],
  suporte: ['face-agent', 'lifebuoy', 'help'],
}

/** Search terms (English and Portuguese) mapped to icon name fragments. */
export const SEARCH_SYNONYMS: Readonly<Record<string, readonly string[]>> = mergeSynonyms(
  EN_SYNONYMS,
  PT_SYNONYMS,
)

function mergeSynonyms(...maps: Array<Record<string, string[]>>): Record<string, string[]> {
  const merged: Record<string, string[]> = {}
  for (const map of maps) {
    for (const [word, names] of Object.entries(map)) {
      merged[word] = [...new Set([...(merged[word] ?? []), ...names])]
    }
  }
  return merged
}

/** Removes accents and lowercases, for accent-insensitive matching. */
export function normalizeQuery(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim()
}

/** Returns the CSS classes for an icon name. */
export function iconClass(name: string): string {
  return `mdi mdi-${name}`
}

/** Extracts the icon name from a stored class ("mdi mdi-home" -> "home"). */
export function iconName(value: string | null | undefined): string | null {
  const match = /(?:^|\s)mdi-([a-z0-9-]+)(?:\s|$)/.exec(value ?? '')
  return match ? match[1]! : null
}

export interface IconSearchResult {
  names: string[]
  total: number
}

/**
 * Searches icon names. Every word of the query must match the name directly or
 * through an English or Portuguese synonym. Names starting with a term rank first.
 */
export function searchIcons(names: readonly string[], query: string, limit = 240): IconSearchResult {
  const words = normalizeQuery(query)
    .split(/[\s-]+/)
    .filter(Boolean)
  if (words.length === 0) return { names: [], total: 0 }

  const synonyms = (word: string) => (Object.hasOwn(SEARCH_SYNONYMS, word) ? SEARCH_SYNONYMS[word]! : [])
  const alternatives = words.map((word) => [word, ...synonyms(word)])
  const scored: Array<{ name: string; score: number }> = []

  for (const name of names) {
    let score = 0
    let matchesAll = true
    for (const terms of alternatives) {
      let best = -1
      for (const term of terms) {
        if (name === term) best = Math.max(best, 3)
        else if (name.startsWith(term)) best = Math.max(best, 2)
        else if (name.includes(term)) best = Math.max(best, 1)
      }
      if (best < 0) {
        matchesAll = false
        break
      }
      score += best
    }
    if (matchesAll) scored.push({ name, score })
  }

  scored.sort((a, b) => b.score - a.score || a.name.length - b.name.length || a.name.localeCompare(b.name))
  return { names: scored.slice(0, limit).map((s) => s.name), total: scored.length }
}
