const COOKIE_MAX_AGE = 60 * 60 * 24 // 24 hours

export function useAuth() {
  const userCookie = useCookie<{ username: string } | null>('auth_user', {
    maxAge: COOKIE_MAX_AGE,
  })
  const user = useState<{ username: string } | null>('auth_user', () => userCookie.value ?? null)
  const isAuthenticated = computed(() => !!user.value)

  const setUser = (newUser: { username: string }) => {
    user.value = newUser
    userCookie.value = newUser
  }

  const clearUser = () => {
    user.value = null
    userCookie.value = null
  }

  const fetchUser = async () => {
    try {
      const { fetchMe } = await import('~/utils/services/auth')
      const me = await fetchMe()
      setUser({ username: me.username })
      return true
    } catch {
      clearUser()
      return false
    }
  }

  const clearToken = async () => {
    try {
      const { logout } = await import('~/utils/services/auth')
      await logout()
    } catch {
      // ignore network errors during logout
    }
    clearUser()
  }

  return {
    user: readonly(user),
    isAuthenticated,
    setUser,
    clearUser,
    clearToken,
    fetchUser,
  }
}
