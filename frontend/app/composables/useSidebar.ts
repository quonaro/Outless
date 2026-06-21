export function useSidebar() {
  const isExpanded = useState('sidebar_isExpanded', () => true)
  const activeItem = useState('sidebar_activeItem', () => 'dashboard')
  const openMenus = useState<Set<string>>('sidebar_openMenus', () => new Set())

  const toggle = () => {
    isExpanded.value = !isExpanded.value
  }

  const setActive = (item: string) => {
    activeItem.value = item
  }

  const toggleMenu = (menuId: string) => {
    const newSet = new Set(openMenus.value)
    if (newSet.has(menuId)) {
      newSet.delete(menuId)
    } else {
      newSet.add(menuId)
    }
    openMenus.value = newSet
  }

  const isMenuOpen = (menuId: string) => {
    return openMenus.value.has(menuId)
  }

  return {
    isExpanded: readonly(isExpanded),
    activeItem: readonly(activeItem),
    openMenus: readonly(openMenus),
    toggle,
    setActive,
    toggleMenu,
    isMenuOpen,
  }
}
