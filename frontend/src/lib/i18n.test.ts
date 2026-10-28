import { afterEach, describe, expect, it, vi } from 'vite-plus/test'

import { findTheme } from '../theme/registry'
import {
  applyLocale,
  currentLocale,
  DICTIONARIES,
  interpolate,
  loadSavedLocale,
  localeMenu,
  LOCALES,
  parseLocaleId,
  resolveLocale,
  saveLocale,
  t,
  translate,
} from './i18n'
import { buildToolbar } from './menu'
import { translateError } from './messages'
import { themeLabel, themeMenu } from './theme'

afterEach(() => {
  currentLocale.value = 'en'
  vi.unstubAllGlobals()
})

describe('dictionaries', () => {
  it('have identical keys and no empty values', () => {
    const english = Object.keys(DICTIONARIES.en).sort()
    for (const { locale } of LOCALES) {
      const dictionary = DICTIONARIES[locale]
      expect(Object.keys(dictionary).sort(), locale).toEqual(english)
      for (const [key, value] of Object.entries(dictionary)) {
        expect(value.trim(), `${locale} ${key}`).not.toBe('')
      }
    }
  })

  it('use the same placeholders in every language', () => {
    const placeholders = (text: string) => [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()
    for (const [key, text] of Object.entries(DICTIONARIES.en)) {
      expect(placeholders(DICTIONARIES['pt-BR'][key as keyof typeof DICTIONARIES.en]), key).toEqual(
        placeholders(text),
      )
    }
  })
})

describe('t', () => {
  it('interpolates named parameters as text', () => {
    expect(interpolate('Open {title} ({n})', { title: '<b>x</b>', n: 2 })).toBe('Open <b>x</b> (2)')
    expect(interpolate('Keep {missing}', {})).toBe('Keep {missing}')
    expect(translate('pt-BR', 'toolbar.open', { title: 'Grafana' })).toBe('Abrir Grafana')
  })

  it('follows the current locale', () => {
    expect(t('toolbar.theme')).toBe('Theme')
    currentLocale.value = 'pt-BR'
    expect(t('toolbar.theme')).toBe('Tema')
    expect(t('dashboard.tabLimit', { limit: 3, title: 'A' })).toBe('Limite de 3 abas: "A" foi fechada')
  })

  it('translates API errors and DHTMLX menus in both languages', () => {
    expect(translateError('invalid password')).toBe('Wrong password.')
    expect(themeMenu({ name: 'dark', style: 'flat' }).value).toBe('Theme')
    currentLocale.value = 'pt-BR'
    expect(translateError('invalid password')).toBe('Senha incorreta.')
    expect(translateError('slug "acme" is reserved')).toBe('O endereço "acme" é reservado.')
    expect(translateError('boom')).toBe('Não foi possível concluir a operação (boom).')
    const menu = themeMenu({ name: 'dark', style: 'flat' })
    expect(menu.value).toBe('Tema')
    expect(menu.items![0]!.value).toBe('Escuros')
    expect(buildToolbar('P', [], [localeMenu()]).find((i) => i.id === 'lang')!.value).toBe('Idioma')
  })

  it('translates theme labels, keeping proper names', () => {
    expect(themeLabel(findTheme('ocean-light')!, 'en')).toBe('Ocean Light')
    expect(themeLabel(findTheme('ocean-light')!, 'pt-BR')).toBe('Oceano Claro')
    expect(themeLabel(findTheme('xp')!, 'pt-BR')).toBe('Windows XP')
  })
})

describe('locale', () => {
  it('resolves the saved choice, then a Portuguese browser, then English', () => {
    expect(resolveLocale('pt-BR', 'en-US')).toBe('pt-BR')
    expect(resolveLocale('en', 'pt-BR')).toBe('en')
    expect(resolveLocale(null, 'pt-BR')).toBe('pt-BR')
    expect(resolveLocale(null, 'pt-PT')).toBe('pt-BR')
    expect(resolveLocale('fr', 'pt')).toBe('pt-BR')
    expect(resolveLocale(undefined, 'en-US')).toBe('en')
    expect(resolveLocale(undefined, 'ptx')).toBe('en')
    expect(resolveLocale(undefined, undefined)).toBe('en')
  })

  it('saves and loads the choice, tolerating unavailable storage', () => {
    expect(loadSavedLocale()).toBeNull()
    const store = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => store.set(k, v),
    })
    saveLocale('pt-BR')
    expect(store.get('dashboard.locale')).toBe('pt-BR')
    expect(loadSavedLocale()).toBe('pt-BR')
    store.set('dashboard.locale', 'xx')
    expect(loadSavedLocale()).toBeNull()
  })

  it('applies the locale to <html lang>', () => {
    const root = { lang: '' } as HTMLElement
    applyLocale('pt-BR', root)
    expect(root.lang).toBe('pt-BR')
    expect(currentLocale.value).toBe('pt-BR')
  })

  it('builds and parses the language menu', () => {
    const menu = localeMenu('pt-BR')
    expect(menu.items!.map((i) => [i.id, i.value, i.css])).toEqual([
      ['lang-en', 'English', undefined],
      ['lang-pt-BR', 'Português (Brasil)', 'menu-current'],
    ])
    expect(parseLocaleId('lang-pt-BR')).toBe('pt-BR')
    expect(parseLocaleId('lang-en')).toBe('en')
    expect(parseLocaleId('lang-fr')).toBeNull()
    expect(parseLocaleId('theme-xp')).toBeNull()
  })
})
