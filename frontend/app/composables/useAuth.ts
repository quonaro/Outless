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
  const token = useState<string | null>('auth_token', () => null)
  const user = useState<{ username: string } | null>('auth_user', () => null)

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
    const cookie = useCookie<string | null>('auth_token', { maxAge: COOKIE_MAX_AGE })
    cookie.value = newToken
  }

  const clearToken = () => {
    token.value = null
    user.value = null
    setLocalStorageValue(TOKEN_LS_KEY, null)
    setLocalStorageValue(USER_LS_KEY, null)
    const cookie = useCookie<string | null>('auth_token', { maxAge: COOKIE_MAX_AGE })
    cookie.value = null
    const userCookie = useCookie<{ username: string } | null>('auth_user', {
      maxAge: COOKIE_MAX_AGE,
    })
    userCookie.value = null
  }

  const setUser = (newUser: { username: string }) => {
    user.value = newUser
    setLocalStorageValue(USER_LS_KEY, JSON.stringify(newUser))
    const userCookie = useCookie<{ username: string } | null>('auth_user', {
      maxAge: COOKIE_MAX_AGE,
    })
    userCookie.value = newUser
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
