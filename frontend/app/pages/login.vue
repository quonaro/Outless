<script setup lang="ts">
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import { nextTick } from 'vue'
import { LogIn } from 'lucide-vue-next'
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
      class="hidden lg:flex lg:w-1/2 relative items-center justify-center overflow-hidden bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900"
    >
      <div
        class="blob blob-1 absolute w-72 h-72 rounded-full bg-purple-500 blur-3xl opacity-30 top-10 left-10"
      ></div>
      <div
        class="blob blob-2 absolute w-96 h-96 rounded-full bg-blue-500 blur-3xl opacity-20 bottom-20 right-10"
      ></div>
      <div
        class="blob blob-3 absolute w-64 h-64 rounded-full bg-cyan-400 blur-3xl opacity-25 top-1/2 left-1/2"
      ></div>

      <div class="relative z-10 flex flex-col items-center gap-6">
        <img src="~/assets/img/logo-d-s.webp" alt="Outless" class="h-20 w-auto drop-shadow-lg" />
        <h1 class="text-4xl font-bold text-white tracking-tight drop-shadow">Outless</h1>
        <p class="text-slate-300 text-lg max-w-xs text-center">Simple. Fast. Secure.</p>
      </div>
    </div>

    <!-- Right side -->
    <div
      class="w-full lg:w-1/2 flex min-h-screen items-center justify-center bg-background px-4 relative"
    >
      <div class="absolute top-4 right-4">
        <ThemeToggle />
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

<style scoped>
@keyframes float {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(30px, -50px) scale(1.1);
  }
  66% {
    transform: translate(-20px, 20px) scale(0.9);
  }
}
.blob {
  animation: float 8s infinite ease-in-out;
}
.blob-1 {
  animation-delay: 0s;
}
.blob-2 {
  animation-delay: 2s;
}
.blob-3 {
  animation-delay: 4s;
}
</style>
