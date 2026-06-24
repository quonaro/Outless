<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { toast } from 'vue-sonner'
import {
  Settings2,
  KeyRound,
  Save,
  Loader2,
  AlertTriangle,
  Shield,
  ShieldOff,
  QrCode,
  Download,
  Upload,
} from 'lucide-vue-next'
import { setupTOTP, verifyTOTP, disableTOTP } from '~/utils/services/auth'
import UiCard from '~/components/ui/card/card.vue'
import CardContent from '~/components/ui/card/CardContent.vue'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiSelect from '~/components/ui/select/select.vue'
import ChangePasswordDialog from '~/components/ui/change-password-dialog/change-password-dialog.vue'
import { useAuth } from '~/composables/useAuth'
import { useConfirm } from '~/composables/useConfirm'
import { useSettings, useUpdateSettings } from '~/composables/settings/useSettings'

const auth = useAuth()
const { confirm } = useConfirm()
const isChangePasswordOpen = ref(false)
const currentLogin = computed(() => auth.user.value?.username ?? 'admin')

const { data: settings, isLoading, isError, error } = useSettings()
const updateSettings = useUpdateSettings()

const LOG_LEVEL_OPTIONS = [
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn' },
  { label: 'Error', value: 'error' },
]

const formDatabase = ref('')
const formHttpPort = ref(41220)
const formLogLevel = ref('info')
const formShutdownGracetime = ref('10s')
const formDisableDocs = ref(false)
const isSaving = ref(false)
const importFileInput = ref<HTMLInputElement | null>(null)

const totpEnabled = ref(false)
const showSetup = ref(false)
const showDisable = ref(false)
const secret = ref('')
const uri = ref('')
const qrBase64 = ref('')
const setupCode = ref('')
const disableCode = ref('')
const disablePassword = ref('')

async function handleSetup() {
  try {
    const response = await setupTOTP()
    secret.value = response.secret
    uri.value = response.uri
    qrBase64.value = response.qr_base64
    showSetup.value = true
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to setup 2FA', { description: msg })
  }
}

async function handleVerify() {
  try {
    await verifyTOTP({ code: setupCode.value })
    totpEnabled.value = true
    showSetup.value = false
    setupCode.value = ''
    toast.success('2FA enabled successfully')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Invalid code', { description: msg })
  }
}

async function handleDisable() {
  try {
    await disableTOTP({ code: disableCode.value, password: disablePassword.value })
    totpEnabled.value = false
    showDisable.value = false
    disableCode.value = ''
    disablePassword.value = ''
    toast.success('2FA disabled')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Failed to disable 2FA', { description: msg })
  }
}

async function handleExport() {
  try {
    const { $api } = useNuxtApp()
    const data = await $api<unknown>('/v1/export')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `outless-config-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    toast.success('Configuration exported')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Export failed', { description: msg })
  }
}

async function handleImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const data = JSON.parse(text)
    const { $api } = useNuxtApp()
    await $api('/v1/import', {
      method: 'POST',
      body: data,
    })
    toast.success('Configuration imported')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error('Import failed', { description: msg })
  } finally {
    input.value = ''
  }
}

watch(
  () => settings.value,
  (s) => {
    if (!s) return
    formDatabase.value = s.database
    formHttpPort.value = s.app.http_port
    formLogLevel.value = s.app.log_level
    formShutdownGracetime.value = s.app.shutdown_gracetime
    formDisableDocs.value = s.app.disable_docs
  },
  { immediate: true }
)

async function handleSave(options?: { danger?: boolean }) {
  if (isSaving.value) return
  const ok = await confirm({
    title: options?.danger ? 'Confirm Dangerous Change' : 'Confirm Save',
    message: options?.danger
      ? 'Changing the HTTP port or database path can make the application unreachable. Proceed?'
      : 'Are you sure you want to save these settings?',
    variant: options?.danger ? 'destructive' : 'default',
    confirmLabel: 'Save',
  })
  if (!ok) return
  isSaving.value = true
  updateSettings.mutate(
    {
      database: formDatabase.value,
      app: {
        http_port: Number(formHttpPort.value),
        log_level: formLogLevel.value,
        shutdown_gracetime: formShutdownGracetime.value,
        disable_docs: formDisableDocs.value,
      },
    },
    {
      onSuccess: () => {
        toast.success('Settings saved')
      },
      onError: (err: Error) => {
        toast.error('Failed to save settings', { description: err.message })
      },
      onSettled: () => {
        isSaving.value = false
      },
    }
  )
}
</script>

<template>
  <div class="space-y-8">
    <div v-if="isLoading" class="py-8 text-center text-muted-foreground">Loading settings...</div>
    <div v-else-if="isError" class="py-8 text-center text-destructive">
      Failed to load settings: {{ error?.message }}
    </div>

    <template v-else>
      <div>
        <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
          <Settings2 class="h-5 w-5 text-primary" />
          Application
        </h2>
        <UiCard class="p-4">
          <CardContent class="p-0 space-y-4">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="space-y-2">
                <label class="text-sm font-medium" for="settings-log-level">Log Level</label>
                <UiSelect
                  id="settings-log-level"
                  v-model="formLogLevel"
                  name="settings-log-level"
                  :options="LOG_LEVEL_OPTIONS"
                />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium" for="settings-gracetime"
                  >Shutdown Gracetime</label
                >
                <UiInput
                  id="settings-gracetime"
                  v-model="formShutdownGracetime"
                  name="settings-gracetime"
                />
              </div>
              <div class="space-y-2 md:col-span-2">
                <label class="text-sm font-medium">API Docs</label>
                <div class="flex items-center gap-2 h-9">
                  <input
                    id="settings-disable-docs"
                    v-model="formDisableDocs"
                    name="settings-disable-docs"
                    type="checkbox"
                    class="h-4 w-4 rounded border-input"
                  />
                  <label for="settings-disable-docs" class="text-sm">Disable Swagger docs</label>
                </div>
              </div>
            </div>
            <div class="flex justify-end pt-2">
              <UiButton :disabled="isSaving" @click="handleSave()">
                <Loader2 v-if="isSaving" class="h-4 w-4 mr-2 animate-spin" />
                <Save v-else class="h-4 w-4 mr-2" />
                {{ isSaving ? 'Saving...' : 'Save Settings' }}
              </UiButton>
            </div>
          </CardContent>
        </UiCard>
      </div>

      <div>
        <h2 class="text-lg font-semibold mb-3 flex items-center gap-2 text-destructive">
          <AlertTriangle class="h-5 w-5 text-destructive" />
          Danger Zone
        </h2>
        <UiCard class="p-4 border-destructive/30">
          <CardContent class="p-0 space-y-4">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="space-y-2">
                <label class="text-sm font-medium" for="settings-http-port">HTTP Port</label>
                <UiInput
                  id="settings-http-port"
                  v-model="formHttpPort"
                  type="number"
                  name="settings-http-port"
                />
                <p class="text-xs text-muted-foreground">Changing this may lock you out.</p>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium" for="settings-database">Database Path</label>
                <UiInput id="settings-database" v-model="formDatabase" name="settings-database" />
                <p class="text-xs text-muted-foreground">Path to the SQLite database file.</p>
              </div>
            </div>
            <div class="flex justify-end pt-2">
              <UiButton
                variant="destructive"
                :disabled="isSaving"
                @click="handleSave({ danger: true })"
              >
                <Loader2 v-if="isSaving" class="h-4 w-4 mr-2 animate-spin" />
                <Save v-else class="h-4 w-4 mr-2" />
                {{ isSaving ? 'Saving...' : 'Save Changes' }}
              </UiButton>
            </div>
          </CardContent>
        </UiCard>
      </div>

      <div>
        <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
          <KeyRound class="h-5 w-5 text-primary" />
          Admin Account
        </h2>
        <UiCard class="p-4">
          <CardContent class="p-0">
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

      <div>
        <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
          <Shield class="h-5 w-5 text-primary" />
          Two-Factor Authentication
        </h2>
        <UiCard class="p-4">
          <CardContent class="p-0 space-y-4">
            <div v-if="!totpEnabled && !showSetup">
              <p class="text-sm text-muted-foreground mb-3">2FA is currently disabled.</p>
              <UiButton @click="handleSetup">
                <QrCode class="h-4 w-4 mr-2" />
                Enable 2FA
              </UiButton>
            </div>

            <div v-else-if="showSetup" class="space-y-4">
              <p class="text-sm text-muted-foreground">
                Scan the QR code with your authenticator app, then enter the code to verify.
              </p>
              <div v-if="qrBase64" class="flex justify-center">
                <img
                  :src="`data:image/png;base64,${qrBase64}`"
                  alt="TOTP QR Code"
                  class="rounded-lg"
                />
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium">Secret (manual entry)</label>
                <code class="block p-2 bg-muted rounded text-sm break-all">{{ secret }}</code>
              </div>
              <div class="space-y-2">
                <label class="text-sm font-medium" for="totp-verify-code">Verification Code</label>
                <UiInput
                  id="totp-verify-code"
                  v-model="setupCode"
                  type="text"
                  inputmode="numeric"
                  maxlength="6"
                  placeholder="6-digit code"
                />
              </div>
              <div class="flex gap-2">
                <UiButton :disabled="!setupCode" @click="handleVerify">Verify & Enable</UiButton>
                <UiButton variant="ghost" @click="showSetup = false">Cancel</UiButton>
              </div>
            </div>

            <div v-else-if="totpEnabled" class="space-y-4">
              <p class="text-sm text-muted-foreground">2FA is enabled.</p>
              <UiButton variant="destructive" @click="showDisable = true">
                <ShieldOff class="h-4 w-4 mr-2" />
                Disable 2FA
              </UiButton>

              <div v-if="showDisable" class="space-y-3 pt-2 border-t">
                <p class="text-sm text-muted-foreground">
                  Enter your password and current TOTP code to disable.
                </p>
                <div class="space-y-2">
                  <label class="text-sm font-medium" for="totp-disable-password">Password</label>
                  <UiInput id="totp-disable-password" v-model="disablePassword" type="password" />
                </div>
                <div class="space-y-2">
                  <label class="text-sm font-medium" for="totp-disable-code">TOTP Code</label>
                  <UiInput
                    id="totp-disable-code"
                    v-model="disableCode"
                    type="text"
                    inputmode="numeric"
                    maxlength="6"
                    placeholder="6-digit code"
                  />
                </div>
                <div class="flex gap-2">
                  <UiButton
                    variant="destructive"
                    :disabled="!disablePassword || !disableCode"
                    @click="handleDisable"
                  >
                    Confirm Disable
                  </UiButton>
                  <UiButton variant="ghost" @click="showDisable = false">Cancel</UiButton>
                </div>
              </div>
            </div>
          </CardContent>
        </UiCard>
      </div>

      <div>
        <h2 class="text-lg font-semibold mb-3 flex items-center gap-2">
          <Download class="h-5 w-5 text-primary" />
          Backup
        </h2>
        <UiCard class="p-4">
          <CardContent class="p-0 space-y-4">
            <p class="text-sm text-muted-foreground">
              Export or import your full configuration (nodes, tokens, groups, inbounds, public
              sources).
            </p>
            <div class="flex gap-2">
              <UiButton @click="handleExport">
                <Download class="h-4 w-4 mr-2" />
                Export
              </UiButton>
              <UiButton variant="outline" @click="importFileInput?.click()">
                <Upload class="h-4 w-4 mr-2" />
                Import
              </UiButton>
              <input
                ref="importFileInput"
                type="file"
                accept=".json"
                class="hidden"
                @change="handleImport"
              />
            </div>
          </CardContent>
        </UiCard>
      </div>
    </template>

    <ChangePasswordDialog v-model:open="isChangePasswordOpen" :current-login="currentLogin" />
  </div>
</template>
