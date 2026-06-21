export function getAuthHeaders(): HeadersInit {
  const token = useCookie<string | null>('auth_token').value
  if (!token) {
    return {}
  }

  return {
    Authorization: `Bearer ${token}`,
  }
}
