// The docs site's theme: VitePress's own, with the playground's component,
// which runs only in the browser and loads only on the page that uses it.

import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import { defineAsyncComponent } from 'vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component(
      'Playground',
      defineAsyncComponent(() => import('./playground/Playground.vue')),
    )
  },
} satisfies Theme
