<script setup lang="ts">
import { ref } from 'vue'
import { useMutation } from '@tanstack/vue-query'
import { ZodError } from 'zod'
import { ChangeAdminPasswordSchema, type ChangeAdminPassword } from '~/utils/schemas/admin'
import { changeAdminPassword } from '~/utils/services/admin'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import Sheet from '~/components/ui/sheet/Sheet.vue'
import SheetContent from '~/components/ui/sheet/SheetContent.vue'
import SheetDescription from '~/components/ui/sheet/SheetDescription.vue'
import SheetFooter from '~/components/ui/sheet/SheetFooter.vue'
import SheetHeader from '~/components/ui/sheet/SheetHeader.vue'
import SheetTitle from '~/components/ui/sheet/SheetTitle.vue'

interface Props {
  open: boolean
  currentLogin: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  success: []
}>()

const formData = ref<ChangeAdminPassword>({
  current_login: props.currentLogin,
  current_password: '',
  new_login: '',
  new_password: '',
  confirm_password: '',
})

const errors = ref<Record<string, string>>({})

const { isPending, mutate } = useMutation({
  mutationFn: (data: ChangeAdminPassword) => changeAdminPassword(data),
  onSuccess: () => {
    emit('success')
    emit('update:open', false)
    resetForm()
  },
  onError: (error: unknown) => {
    const apiError = error as { data?: { message?: string } }
    if (apiError.data?.message) {
      errors.value.current_password = apiError.data.message
    } else {
      errors.value.current_password = 'Failed to change password'
    }
  },
})

function resetForm() {
  formData.value = {
    current_login: props.currentLogin,
    current_password: '',
    new_login: '',
    new_password: '',
    confirm_password: '',
  }
  errors.value = {}
}

function handleSubmit() {
  errors.value = {}

  try {
    ChangeAdminPasswordSchema.parse(formData.value)
    mutate(formData.value)
  } catch (err: unknown) {
    if (err instanceof ZodError) {
      err.errors.forEach((error) => {
        const path = error.path[0] as string
        errors.value[path] = error.message
      })
    }
  }
}

function handleOpenChange(value: boolean) {
  if (value) {
    formData.value.current_login = props.currentLogin
  }
  if (!value) {
    resetForm()
  }
  emit('update:open', value)
}
</script>

<template>
  <Sheet :open="open" @update:open="handleOpenChange">
    <SheetContent class="sm:max-w-[500px]">
      <SheetHeader>
        <SheetTitle>Change Admin Password</SheetTitle>
        <SheetDescription>
          Enter your current credentials and new password to change admin access.
        </SheetDescription>
      </SheetHeader>

      <div class="space-y-4 py-4">
        <div class="space-y-2">
          <label class="text-sm font-medium">Current Login</label>
          <UiInput
            v-model="formData.current_login"
            :placeholder="currentLogin"
            type="text"
            :class="{ 'border-red-500': errors.current_login }"
          />
          <span v-if="errors.current_login" class="text-sm text-red-500">{{
            errors.current_login
          }}</span>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">Current Password</label>
          <UiInput
            v-model="formData.current_password"
            type="password"
            placeholder="Enter current password"
            :class="{ 'border-red-500': errors.current_password }"
          />
          <span v-if="errors.current_password" class="text-sm text-red-500">{{
            errors.current_password
          }}</span>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">New Login (Optional)</label>
          <UiInput
            v-model="formData.new_login"
            type="text"
            placeholder="Leave empty to keep current"
            :class="{ 'border-red-500': errors.new_login }"
          />
          <span v-if="errors.new_login" class="text-sm text-red-500">{{ errors.new_login }}</span>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">New Password</label>
          <UiInput
            v-model="formData.new_password"
            type="password"
            placeholder="Min 8 characters"
            :class="{ 'border-red-500': errors.new_password }"
          />
          <span v-if="errors.new_password" class="text-sm text-red-500">{{
            errors.new_password
          }}</span>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">Confirm New Password</label>
          <UiInput
            v-model="formData.confirm_password"
            type="password"
            placeholder="Confirm new password"
            :class="{ 'border-red-500': errors.confirm_password }"
          />
          <span v-if="errors.confirm_password" class="text-sm text-red-500">{{
            errors.confirm_password
          }}</span>
        </div>
      </div>

      <SheetFooter>
        <UiButton variant="outline" @click="emit('update:open', false)"> Cancel </UiButton>
        <UiButton :disabled="isPending" @click="handleSubmit">
          {{ isPending ? 'Changing...' : 'Change Password' }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
