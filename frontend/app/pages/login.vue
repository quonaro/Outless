<script setup lang="ts">
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import { nextTick } from 'vue'
import { LogIn, Github, ArrowRight } from 'lucide-vue-next'
import { login } from '~/utils/services/auth'
import { useAuth } from '~/composables/useAuth'

definePageMeta({
  layout: false,
})

useHead({
  title: 'Login',
})

const FormSchema = toTypedSchema(
  z.object({
    username: z.string().min(1, 'Username is required'),
    password: z.string().min(1, 'Password is required'),
  })
)

const { handleSubmit, errors, defineField } = useForm({
  validationSchema: FormSchema,
})

const [username] = defineField('username')
const [password] = defineField('password')

const auth = useAuth()
const isLoading = ref(false)
const errorMessage = ref('')

const onSubmit = handleSubmit(async (values) => {
  isLoading.value = true
  errorMessage.value = ''

  try {
    console.log('[LOGIN] Starting login with username:', values.username)
    const response = await login(values)
    console.log('[LOGIN] Login response received:', response)
    auth.setToken(response.token)
    console.log('[LOGIN] Token set, current token:', auth.token.value)
    auth.setUser({ username: response.username })
    console.log('[LOGIN] User set, current user:', auth.user.value)
    console.log('[LOGIN] isAuthenticated:', auth.isAuthenticated.value)
    await nextTick()
    console.log('[LOGIN] nextTick completed, navigating to /dashboard')
    await navigateTo('/dashboard')
    console.log('[LOGIN] navigateTo completed')
  } catch (error) {
    console.error('[LOGIN] Login error:', error)
    errorMessage.value = 'Invalid username or password'
  } finally {
    isLoading.value = false
  }
})

onMounted(async () => {
  if (auth.isAuthenticated.value) {
    await navigateTo('/dashboard')
  }
})
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Left side -->
    <div
      class="hidden lg:flex lg:w-1/2 relative flex-col justify-center items-center overflow-hidden bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 px-12 py-16"
    >
      <NetworkBackground />

      <div class="relative z-10 flex flex-col items-center gap-8">
        <div class="max-w-md text-center">
          <h2 class="text-5xl font-bold text-white leading-tight">
            Proxy management, <span class="text-blue-500">redefined</span>
          </h2>
          <p class="mt-6 text-slate-400 text-lg leading-relaxed">
            Deploy, monitor, and scale your proxy servers from one unified dashboard. Built for
            speed, engineered for reliability.
          </p>
        </div>

        <a
          href="https://github.com/OutVless/Outless"
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center gap-2 px-5 py-2.5 rounded-lg border border-slate-600 text-slate-300 hover:text-white hover:border-slate-400 transition-colors"
        >
          <Github class="h-5 w-5" />
          <span class="text-sm">View on GitHub</span>
          <ArrowRight class="h-4 w-4 ml-1" />
        </a>
      </div>

      <div class="absolute bottom-6 left-0 right-0 text-center text-slate-500 text-sm">
        © 2026 Outless. Open source.
      </div>
    </div>

    <!-- Right side -->
    <div
      class="w-full lg:w-1/2 flex min-h-screen flex-col items-center justify-center bg-background px-4 relative"
    >
      <div class="absolute top-4 right-4">
        <ThemeToggle />
      </div>

      <div class="flex items-center gap-4 mb-10">
        <img src="~/assets/img/logo-d-s.webp" alt="Outless" class="h-16 w-auto" />
        <h1 class="text-4xl font-bold text-foreground tracking-tight">Outless</h1>
      </div>

      <Card class="w-full max-w-md bg-card border-border">
        <CardHeader class="space-y-1">
          <CardTitle class="text-2xl font-bold text-center text-card-foreground">
            Admin Login
          </CardTitle>
          <CardDescription class="text-center text-muted-foreground">
            Enter your credentials to access the admin panel
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form class="space-y-4" @submit="onSubmit">
            <div class="space-y-2">
              <Label for="username" class="text-foreground">Username</Label>
              <Input
                id="username"
                v-model="username"
                type="text"
                autocomplete="username"
                class="bg-input border-border text-foreground placeholder:text-muted-foreground"
              />
              <p v-if="errors.username" class="text-sm text-destructive">
                {{ errors.username }}
              </p>
            </div>

            <div class="space-y-2">
              <Label for="password" class="text-foreground">Password</Label>
              <Input
                id="password"
                v-model="password"
                type="password"
                autocomplete="current-password"
                class="bg-input border-border text-foreground placeholder:text-muted-foreground"
              />
              <p v-if="errors.password" class="text-sm text-destructive">
                {{ errors.password }}
              </p>
            </div>

            <p v-if="errorMessage" class="text-sm text-destructive text-center">
              {{ errorMessage }}
            </p>

            <Button
              type="submit"
              class="w-full bg-primary text-primary-foreground hover:bg-primary/90"
              :disabled="isLoading"
            >
              <LogIn v-if="!isLoading" class="mr-2 h-4 w-4" />
              <span v-if="isLoading">Signing in...</span>
              <span v-else>Sign in</span>
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
