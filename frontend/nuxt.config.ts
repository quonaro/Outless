// https://nuxt.com/docs/api/configuration/nuxt-config

export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',

  devtools: {
    enabled: process.env.NODE_ENV === 'production' ? false : process.env.NUXT_DEVTOOLS !== 'false',
  },

  future: {
    compatibilityVersion: 4,
  },

  app: {
    head: {
      title: 'Outless',
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon/favicon.ico' },
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon/favicon.svg' },
        { rel: 'apple-touch-icon', sizes: '180x180', href: '/favicon/apple-touch-icon.png' },
        { rel: 'manifest', href: '/favicon/site.webmanifest' },
      ],
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
    },
  },

  modules: ['@nuxtjs/color-mode', '@vueuse/nuxt', 'shadcn-nuxt', '@nuxt/eslint'],

  shadcn: {
    prefix: '',
    componentDir: 'app/components/ui',
  },

  colorMode: {
    preference: 'system',
    fallback: 'light',
    classSuffix: '',
    storageKey: 'outless-theme',
  },

  css: ['./app/assets/css/main.css'],

  runtimeConfig: {
    public: {
      // DEV: use relative /api (proxied by vite to current host)
      // PROD: use full URL from NUXT_PUBLIC_API_BASE env var
      apiBase: (() => {
        const apiBase =
          process.env.NODE_ENV === 'development'
            ? '/api'
            : process.env.NUXT_PUBLIC_API_BASE || '/api'
        return apiBase
      })(),
    },
  },

  devServer: {
    port: 41221,
    host: '127.0.0.1',
  },

  typescript: {
    strict: true,
    typeCheck: false,
  },

  postcss: {
    plugins: {
      tailwindcss: {},
      autoprefixer: {},
    },
  },

  vite: {
    optimizeDeps: {
      include: [
        '@tanstack/vue-query',
        'vee-validate',
        '@vee-validate/zod',
        'zod',
        'lucide-vue-next',
        'class-variance-authority',
        'reka-ui',
        'clsx',
        'tailwind-merge',
        'radix-vue',
        'shadcn-nuxt',
        'tailwindcss-animate',
        '@vue/devtools-core',
        '@vue/devtools-kit',
      ],
      exclude: [],
    },
    resolve: {
      alias: {},
    },
    server: {
      allowedHosts: ['x.locrun.su'],
      headers: {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
        'Access-Control-Allow-Headers': '*',
      },
      proxy: {
        '/api': {
          target: 'http://localhost:41220',
          changeOrigin: true,
        },
      },
    },
  },
})
