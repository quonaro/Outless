import { useColorMode } from '@vueuse/core'

export type Theme = 'light' | 'dark' | 'system'

export function useTheme() {
  const theme = useState<Theme>('theme', () => 'system')

  const colorMode = useColorMode({
    attribute: 'class',
    selector: 'html',
  })

  const isDark = computed(() => {
    if (theme.value === 'system') {
      return colorMode.value === 'dark'
    }
    return theme.value === 'dark'
  })

  const setTheme = (newTheme: Theme) => {
    theme.value = newTheme

    if (newTheme === 'system') {
      colorMode.value = 'auto'
    } else {
      colorMode.value = newTheme
    }
  }

  const toggleTheme = () => {
    if (theme.value === 'light') {
      setTheme('dark')
    } else if (theme.value === 'dark') {
      setTheme('light')
    } else {
      // system preference
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
