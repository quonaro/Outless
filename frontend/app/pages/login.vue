<script setup lang="ts">
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import { z } from "zod";
import { nextTick } from "vue";
import { LogIn } from "lucide-vue-next";
import { login } from "~/utils/services/auth";
import { useAuth } from "~/composables/useAuth";

definePageMeta({
  layout: false,
});

const FormSchema = toTypedSchema(
  z.object({
    username: z.string().min(1, "Username is required"),
    password: z.string().min(1, "Password is required"),
  }),
);

const { handleSubmit, errors, defineField } = useForm({
  validationSchema: FormSchema,
});

const [username] = defineField("username");
const [password] = defineField("password");

const auth = useAuth();
const isLoading = ref(false);
const errorMessage = ref("");

const onSubmit = handleSubmit(async (values) => {
  isLoading.value = true;
  errorMessage.value = "";

  try {
    console.log("[LOGIN] Starting login with username:", values.username);
    const response = await login(values);
    console.log("[LOGIN] Login response received:", response);
    auth.setToken(response.token);
    console.log("[LOGIN] Token set, current token:", auth.token.value);
    auth.setUser({ username: response.username });
    console.log("[LOGIN] User set, current user:", auth.user.value);
    console.log("[LOGIN] isAuthenticated:", auth.isAuthenticated.value);
    await nextTick();
    console.log("[LOGIN] nextTick completed, navigating to /dashboard");
    await navigateTo("/dashboard");
    console.log("[LOGIN] navigateTo completed");
  } catch (error) {
    console.error("[LOGIN] Login error:", error);
    errorMessage.value = "Invalid username or password";
  } finally {
    isLoading.value = false;
  }
});

onMounted(async () => {
  if (auth.isAuthenticated.value) {
    await navigateTo("/dashboard");
  }
});
</script>

<template>
  <div
    class="flex min-h-screen items-center justify-center bg-background px-4 relative"
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
              placeholder="admin"
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
              placeholder="••••••••"
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
</template>
