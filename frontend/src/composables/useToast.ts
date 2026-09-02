import { ref } from 'vue'

interface ToastItem {
  id: number
  message: string
  kind: 'success' | 'danger' | 'info'
}

const toasts = ref<ToastItem[]>([])
let nextId = 0

function push(message: string, kind: ToastItem['kind'] = 'info', durationMs = 3200) {
  const id = ++nextId
  toasts.value.push({ id, message, kind })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, durationMs)
}

function ok(message: string) {
  push(message, 'success')
}
function err(message: string) {
  push(message, 'danger', 4500)
}

export function useToast() {
  return { toasts, push, ok, err }
}