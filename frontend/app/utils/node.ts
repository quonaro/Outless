export function resolveCreateNodeErrorMessage(error: unknown): string {
  const statusCode = Number((error as { statusCode?: unknown })?.statusCode)
  if (statusCode === 409) {
    return 'A node with this identifier already exists.'
  }

  const data = (error as { data?: unknown })?.data as
    | { message?: unknown; detail?: unknown; title?: unknown }
    | undefined
  if (typeof data?.message === 'string' && data.message.trim()) return data.message
  if (typeof data?.detail === 'string' && data.detail.trim()) return data.detail
  if (typeof data?.title === 'string' && data.title.trim()) return data.title

  const message = (error as { message?: unknown })?.message
  if (typeof message === 'string' && message.trim()) return message
  return 'Failed to create node.'
}
