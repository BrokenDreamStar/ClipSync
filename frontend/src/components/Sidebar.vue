<script setup lang="ts">
import Icon from './Icon.vue'
import { useAppState } from '../composables/useAppState'

defineProps<{ tabs: { key: string; label: string; icon: string }[]; active: string }>()
const emit = defineEmits<{ (e: 'select', key: string): void }>()

const state = useAppState()
</script>

<template>
  <aside class="drag-region shrink-0 w-[200px] flex flex-col select-none bg-bg">
    <nav class="flex-1 px-3 space-y-1 pt-4">
      <button
        v-for="t in tabs"
        :key="t.key"
        class="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition cursor-pointer leading-none"
        :class="
          t.key === active
            ? 'bg-white/[0.05] text-text'
            : 'text-muted hover:text-text hover:bg-white/[0.03]'
        "
        @click="emit('select', t.key)"
      >
        <Icon :name="t.icon" :size="17" :class="t.key === active ? 'text-accent' : ''" />
        {{ t.label }}
        <span
          v-if="t.key === 'devices' && state.pairRequests.value.length"
          class="ml-auto text-[10px] font-semibold text-bg bg-success rounded-full px-1.5 min-w-[18px] h-[18px] inline-flex items-center justify-center leading-none"
        >
          {{ state.pairRequests.value.length }}
        </span>
      </button>
    </nav>

    <div class="px-6 pt-3 pb-6 flex items-center gap-2 min-w-0">
      <span
        class="w-1.5 h-1.5 rounded-full shrink-0"
        :class="state.connected.value.length ? 'bg-success animate-pulse-soft' : 'bg-muted/60'"
      />
      <span class="text-[11px] leading-none text-muted truncate">{{ state.onlineLabel.value }}</span>
    </div>
  </aside>
</template>
