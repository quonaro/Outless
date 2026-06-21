<script setup lang="ts">
import { computed } from 'vue'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import { useStats } from '~/composables/stats/useStats'

definePageMeta({
  layout: 'default',
})

const { data: stats, isLoading, isError, error } = useStats()

interface StatCard {
  label: string
  value: number
  hint?: string
}

const cards = computed<StatCard[]>(() => {
  const s = stats.value
  if (!s) return []
  return [
    { label: 'Total nodes', value: s.nodes_total },
    { label: 'Active tokens', value: s.tokens_active, hint: `${s.tokens_total} total` },
    { label: 'Groups', value: s.groups_total },
  ]
})
</script>

<template>
  <UiPageLayout title="Dashboard" description="Overview of Outless state">
    <ClientOnly>
      <template #fallback>
        <div class="py-8 text-center text-muted-foreground">
          Loading stats...
        </div>
      </template>

      <div v-if="isLoading" class="py-8 text-center text-muted-foreground">
        Loading stats...
      </div>
      <div v-else-if="isError" class="py-8 text-center text-destructive">
        Failed to load stats: {{ error?.message }}
      </div>
      <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <UiCard v-for="card in cards" :key="card.label" class="p-4">
          <CardContent class="p-0 space-y-2">
            <p class="text-sm text-muted-foreground">{{ card.label }}</p>
            <p class="text-3xl font-semibold">{{ card.value }}</p>
            <p v-if="card.hint" class="text-xs text-muted-foreground">
              {{ card.hint }}
            </p>
          </CardContent>
        </UiCard>
      </div>
    </ClientOnly>
  </UiPageLayout>
</template>
