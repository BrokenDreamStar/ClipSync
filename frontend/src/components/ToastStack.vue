<script setup lang="ts">
import Icon from './Icon.vue'

defineProps<{ toasts: { id: number; message: string; kind: 'success' | 'danger' | 'info' }[] }>()

const iconFor = (kind: string) => (kind === 'success' ? 'check-circle' : kind === 'danger' ? 'alert' : 'info')
</script>

<template>
  <div class="fixed bottom-5 right-5 flex flex-col gap-2 z-50 max-w-sm">
    <transition-group name="toast" tag="div" class="flex flex-col gap-2">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="toast-card"
        :class="{ 'toast-success': t.kind === 'success', 'toast-danger': t.kind === 'danger', 'toast-info': t.kind === 'info' }"
      >
        <span class="toast-icon">
          <Icon :name="iconFor(t.kind)" :size="16" />
        </span>
        <span class="flex-1 break-words">{{ t.message }}</span>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 14px;
  border-radius: 12px;
  background: rgba(18, 21, 31, 0.92);
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
  color: #eaedf4;
  text-align: left;
}
.toast-icon {
  flex: none;
  margin-top: 1px;
}
.toast-success {
  border-color: rgba(52, 211, 153, 0.34);
}
.toast-success .toast-icon {
  color: #34d399;
}
.toast-danger {
  border-color: rgba(248, 113, 113, 0.34);
}
.toast-danger .toast-icon {
  color: #f87171;
}
.toast-info {
  border-color: rgba(129, 140, 248, 0.34);
}
.toast-info .toast-icon {
  color: #818cf8;
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
