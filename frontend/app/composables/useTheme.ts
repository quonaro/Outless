export type Theme = 'light' | 'dark' | 'system'

interface ColorModeInstance {
  preference: Theme
  value: 'light' | 'dark'
  unknown: boolean
  forced: boolean
}

export function useTheme() {
  const colorMode = useNuxtApp().$colorMode as ColorModeInstance

  const theme = computed<Theme>(() => colorMode.preference)

  const isDark = computed(() => colorMode.value === 'dark')

  const setTheme = (newTheme: Theme) => {
    colorMode.preference = newTheme
  }

  const toggleTheme = () => {
    if (theme.value === 'light') {
      setTheme('dark')
    } else if (theme.value === 'dark') {
      setTheme('light')
    } else {
      setTheme(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'light' : 'dark')
    }
  }

  return {
    theme: readonly(theme),
    isDark,
    setTheme,
    toggleTheme,
  }
}
