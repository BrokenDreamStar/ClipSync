<script setup lang="ts">
import { ref, computed } from 'vue'
import TitleBar from './components/TitleBar.vue'
import Sidebar from './components/Sidebar.vue'
import HistoryView from './components/HistoryView.vue'
import DevicesView from './components/DevicesView.vue'
import SettingsView from './components/SettingsView.vue'
import AboutView from './components/AboutView.vue'
import ToastStack from './components/ToastStack.vue'
import { useToast } from './composables/useToast'
import { useTheme } from './composables/useTheme'

const tabs = [
  { key: 'history', label: '历史', icon: 'clipboard', component: HistoryView },
  { key: 'devices', label: '设备', icon: 'users', component: DevicesView },
  { key: 'settings', label: '设置', icon: 'sliders', component: SettingsView },
  { key: 'about', label: '关于', icon: 'info', component: AboutView },
]

const activeKey = ref('history')
const activeComponent = computed(() => tabs.find((t) => t.key === activeKey.value)!.component)

const toast = useToast()
// 主题以 Go 侧配置为唯一事实来源，在这里绑定一次即可全局生效。
useTheme()
</script>

<template>
  <div class="h-full flex flex-col">
    <TitleBar />

    <div class="flex-1 flex min-h-0">
      <Sidebar :tabs="tabs" :active="activeKey" @select="activeKey = $event" />

      <main class="flex-1 min-w-0 overflow-y-auto bg-surface">
        <div class="mx-auto max-w-2xl px-6 pt-6">
          <transition name="page" mode="out-in">
            <KeepAlive>
              <component :is="activeComponent" />
            </KeepAlive>
          </transition>
        </div>
      </main>
    </div>

    <ToastStack :toasts="toast.toasts.value" />
  </div>
</template>

<style scoped>
.page-enter-active,
.page-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
