<script setup lang="ts">
import Icon from './Icon.vue'

defineProps<{
  open: boolean
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}>()

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()
</script>

<template>
  <Teleport to="body">
    <transition name="fade">
      <div v-if="open" class="fixed inset-0 z-[60] flex items-center justify-center p-6">
        <div class="absolute inset-0 bg-scrim backdrop-blur-sm" @click="emit('cancel')"></div>
        <div
          class="relative w-full max-w-sm rounded-2xl bg-elevated border border-line shadow-pop overflow-hidden"
          role="dialog"
          aria-modal="true"
        >
          <div class="p-5">
            <div class="flex items-start gap-3">
              <div
                class="w-10 h-10 rounded-xl inline-flex items-center justify-center shrink-0"
                :class="danger ? 'bg-danger/15 text-danger' : 'bg-accent/15 text-accent'"
              >
                <Icon :name="danger ? 'alert' : 'info'" :size="20" />
              </div>
              <div class="min-w-0">
                <h3 v-if="title" class="text-text text-[15px] font-semibold leading-snug">{{ title }}</h3>
                <p class="text-muted text-sm mt-1 break-words">{{ message }}</p>
              </div>
            </div>
          </div>
          <div class="flex gap-2 px-5 pb-5 pt-1">
            <button class="btn btn-ghost flex-1" @click="emit('cancel')">{{ cancelText || '取消' }}</button>
            <button class="btn flex-1" :class="danger ? 'btn-danger' : 'btn-primary'" @click="emit('confirm')">
              {{ confirmText || '确认' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
