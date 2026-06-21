const COOKIE_MAX_AGE = 60 * 60 * 24 // 24 hours
const TOKEN_LS_KEY = 'auth_token'
const USER_LS_KEY = 'auth_user'

function getLocalStorageValue(key: string): string | null {
  if (typeof window === 'undefined') return null
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function setLocalStorageValue(key: string, value: string | null) {
  if (typeof window === 'undefined') return
  try {
    if (value === null) {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, value)
    }
  } catch {
    // ignore
  }
}

export function useAuth() {
  const token = useCookie<string | null>('auth_token', {
    default: () => null,
    maxAge: COOKIE_MAX_AGE,
  })

  const user = useCookie<{ username: string } | null>('auth_user', {
    default: () => null,
    maxAge: COOKIE_MAX_AGE,
  })

  // Sync with localStorage on client init
  if (import.meta.client && !token.value) {
    const lsToken = getLocalStorageValue(TOKEN_LS_KEY)
    if (lsToken) {
      token.value = lsToken
    }
  }
  if (import.meta.client && !user.value) {
    const lsUser = getLocalStorageValue(USER_LS_KEY)
    if (lsUser) {
      try {
        user.value = JSON.parse(lsUser)
      } catch {
        // ignore
      }
    }
  }

  const isAuthenticated = computed(() => !!token.value)

  const setToken = (newToken: string) => {
    token.value = newToken
    setLocalStorageValue(TOKEN_LS_KEY, newToken)
  }

  const clearToken = () => {
    token.value = null
    user.value = null
    setLocalStorageValue(TOKEN_LS_KEY, null)
    setLocalStorageValue(USER_LS_KEY, null)
  }

  const setUser = (newUser: { username: string }) => {
    user.value = newUser
    setLocalStorageValue(USER_LS_KEY, JSON.stringify(newUser))
  }

  return {
    token: readonly(token),
    user: readonly(user),
    isAuthenticated,
    setToken,
    clearToken,
    setUser,
  }
}
