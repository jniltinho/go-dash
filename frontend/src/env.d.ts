/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'virtual:mdi-icons' {
  /** Material Design Icons names (without the "mdi-" prefix). */
  const names: string[]
  export default names
}
