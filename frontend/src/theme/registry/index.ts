import { bioManager } from './bio-manager'
import { oceanDark, oceanLight } from './ocean'
import { blueBlack } from './blue-black'
import { brasil } from './brasil'
import { dark } from './dark'
import { google } from './google'
import { graphite } from './graphite'
import { highContrast } from './high-contrast'
import { light } from './light'
import { linuxKde } from './linux-kde'
import { macClassic } from './mac-classic'
import { macos } from './macos'
import { matrix } from './matrix'
import { microsoft } from './microsoft'
import { midnight } from './midnight'
import { superMarioGalaxy } from './super-mario-galaxy'
import type { ThemeDefinition, ThemeGroup } from './types'
import { ubuntu } from './ubuntu'
import { windows98 } from './windows-98'
import { windows10 } from './windows-10'
import { windows7 } from './windows-7'
import { windowsVista } from './windows-vista'
import { windowsXP } from './windows-xp'
import { winamp } from './winamp'
import { xterm } from './xterm'

export type { ThemeColors, ThemeDefinition, ThemeGradients, ThemeGroup } from './types'

/** Registered themes, in menu order. The first one is the default. */
export const THEMES: readonly ThemeDefinition[] = [
  blueBlack,
  dark,
  graphite,
  midnight,
  light,
  oceanLight,
  oceanDark,
  bioManager,
  google,
  microsoft,
  superMarioGalaxy,
  brasil,
  winamp,
  windows10,
  windows7,
  windowsVista,
  windowsXP,
  windows98,
  macos,
  macClassic,
  ubuntu,
  linuxKde,
  xterm,
  matrix,
  highContrast,
]

/** Theme groups, in menu order; labels come from the i18n dictionaries (`themeGroup.<group>`). */
export const THEME_GROUPS: ReadonlyArray<{ group: ThemeGroup }> = [
  { group: 'dark' },
  { group: 'light' },
  { group: 'brand' },
  { group: 'inspired' },
  { group: 'system' },
  { group: 'special' },
]

/** Former theme names still accepted (e.g. a choice saved in a browser before a rename). */
export const THEME_ALIASES: Readonly<Record<string, string>> = {
  'ad-manager': 'bio-manager',
  'super-mario-world': 'super-mario-galaxy',
}

export function findTheme(name: string): ThemeDefinition | undefined {
  const id = Object.hasOwn(THEME_ALIASES, name) ? THEME_ALIASES[name]! : name
  return THEMES.find((t) => t.name === id)
}
