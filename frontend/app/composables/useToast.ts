import { toast } from 'vue-sonner'

export const useToast = () => {
  const success = (message: string, description?: string) => {
    toast.success(message, {
      description,
    })
  }

  const error = (message: string, description?: string) => {
    toast.error(message, {
      description,
    })
  }

  const info = (message: string, description?: string) => {
    toast(message, {
      description,
    })
  }

  const warning = (message: string, description?: string) => {
    toast.warning(message, {
      description,
    })
  }

  const promise = <T>(
    promise: Promise<T>,
    {
      loading,
      success,
      error,
    }: {
      loading: string
      success: string | ((data: T) => string)
      error: string | ((error: Error) => string)
    }
  ) => {
    return toast.promise(promise, {
      loading,
      success,
      error,
    })
  }

  return {
    success,
    error,
    info,
    warning,
    promise,
  }
}
