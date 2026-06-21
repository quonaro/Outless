<script setup lang="ts">
import { Settings2, KeyRound } from 'lucide-vue-next'
import UiPageLayout from '~/components/ui/page-layout/page-layout.vue'
import UiCard from '~/components/ui/card/card.vue'
import CardHeader from '~/components/ui/card/CardHeader.vue'
import CardTitle from '~/components/ui/card/CardTitle.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import UiButton from '~/components/ui/button/button.vue'
import ChangePasswordDialog from '~/components/ui/change-password-dialog/change-password-dialog.vue'
import { useAuth } from '~/composables/useAuth'

useHead({
  title: 'Settings',
})

const auth = useAuth()
const isChangePasswordOpen = ref(false)

const currentLogin = computed(() => auth.user.value?.username ?? 'admin')
</script>

<template>
  <UiPageLayout title="Settings" description="Configure server and routing options">
    <div class="max-w-2xl space-y-6">
      <UiCard>
        <CardHeader>
          <div class="flex items-center gap-2">
            <Settings2 class="h-5 w-5 text-muted-foreground" />
            <CardTitle>Server Settings</CardTitle>
          </div>
        </CardHeader>
        <CardContent class="space-y-6">
          <p class="text-sm text-muted-foreground">
            Server settings are managed through the configuration file. Inbounds are configured in
            the
            <NuxtLink to="/inbounds" class="text-primary hover:underline">Inbounds</NuxtLink>
            section.
          </p>
        </CardContent>
      </UiCard>

      <UiCard>
        <CardHeader>
          <div class="flex items-center gap-2">
            <KeyRound class="h-5 w-5 text-muted-foreground" />
            <CardTitle>Admin Account</CardTitle>
          </div>
        </CardHeader>
        <CardContent class="space-y-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p class="text-sm font-medium text-foreground">Login</p>
              <p class="text-sm text-muted-foreground">{{ currentLogin }}</p>
            </div>
            <UiButton class="shrink-0" @click="isChangePasswordOpen = true">
              Change Password
            </UiButton>
          </div>
        </CardContent>
      </UiCard>
    </div>

    <ChangePasswordDialog v-model:open="isChangePasswordOpen" :current-login="currentLogin" />
  </UiPageLayout>
</template>
