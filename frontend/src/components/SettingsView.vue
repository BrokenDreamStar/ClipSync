<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import Icon from './Icon.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { SaveConfig, GetAutostart, SetAutostart } from '../wailsjs/go/main/App'
import { useAppState } from '../composables/useAppState'
import { useToast } from '../composables/useToast'
import { themeOptions, setThemePref, themePref, type ThemePref } from '../composables/useTheme'

const state = useAppState()
const toast = useToast()

const name = ref('')
const port = ref(9250)

watch(
  () => state.cfg.value,
  (c) => {
    if (!c) return
    name.value = c.name
    port.value = c.port
  },
  { immediate: true }
)

const deviceID = computed(() => state.cfg.value?.device_id ?? '')

async function onSave() {
  const err = await SaveConfig(name.value, Number(port.value))
  if (err) {
    toast.err('保存失败：' + err)
    return
  }
  toast.ok('配置已保存。修改本机名或端口需重启生效。')
}

// ---- 外观 ----

async function onTheme(t: ThemePref) {
  const err = await setThemePref(t)
  if (err) {
    toast.err('主题切换失败：' + err)
  }
}

// ---- 开机自启 ----

const autostart = ref(false)
const autostartLoading = ref(true)

onMounted(async () => {
  try {
    autostart.value = await GetAutostart()
  } finally {
    autostartLoading.value = false
  }
})

async function onAutostart(enable: boolean) {
  const err = await SetAutostart(enable)
  if (err) {
    // 写系统设置失败：回滚开关状态并提示。
    autostart.value = !enable
    toast.err('设置开机自启失败：' + err)
    return
  }
  toast.ok(enable ? '已开启开机自启，下次登录生效' : '已关闭开机自启')
}
</script>

<template>
  <div class="pb-4">
    <header class="mb-10">
      <h1 class="text-[22px] font-semibold text-text leading-tight">设置</h1>
      <p class="text-sm text-muted mt-1">本机信息与同步参数</p>
    </header>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">本机信息</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <label class="block">
          <span class="field-label">本机名</span>
          <input v-model="name" class="input" placeholder="如 my-mac" />
        </label>
        <label class="block">
          <span class="field-label">端口</span>
          <input v-model.number="port" type="number" class="input font-mono" />
        </label>
      </div>
      <p class="text-xs text-muted mt-4 break-all">
        设备 ID：<span class="font-mono">{{ deviceID }}</span>
      </p>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">外观</h2>
      <span class="field-label">主题</span>
      <div class="seg" role="radiogroup" aria-label="主题">
        <button
          v-for="o in themeOptions"
          :key="o.value"
          class="seg-btn"
          :class="{ active: themePref === o.value }"
          role="radio"
          :aria-checked="themePref === o.value"
          @click="onTheme(o.value)"
        >
          <Icon :name="o.icon" :size="15" />
          {{ o.label }}
        </button>
      </div>
      <p class="text-xs text-muted mt-3">「跟随系统」下应用外观随系统深浅色自动切换。</p>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">通用</h2>
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm text-text">开机自启</p>
          <p class="text-xs text-muted mt-1">登录系统后自动在后台启动 ClipSync</p>
        </div>
        <ToggleSwitch v-model="autostart" :disabled="autostartLoading" @update:model-value="onAutostart" />
      </div>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-3">配对方式</h2>
      <p class="text-sm text-muted leading-relaxed">
        在「设备」页点击「配对」发起请求，对方确认后即开始同步；无需在两端配置相同的密钥。
      </p>
    </section>

    <div class="flex items-center justify-between gap-4">
      <p class="text-xs text-muted">修改本机名或端口需重启生效</p>
      <button class="btn btn-primary" @click="onSave">
        <Icon name="check" :size="16" />
        保存配置
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 分段控件：井底用背景色，选中项浮起为 elevated 面。 */
.seg {
  display: inline-flex;
  padding: 3px;
  gap: 2px;
  border-radius: 12px;
  background: rgb(var(--c-bg-rgb) / 0.6);
  border: 1px solid var(--c-hairline);
}
.seg-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 14px;
  border-radius: 9px;
  border: none;
  background: transparent;
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  color: rgb(var(--c-muted-rgb));
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease, box-shadow 0.15s ease;
}
.seg-btn:hover {
  color: rgb(var(--c-text-rgb));
}
.seg-btn.active {
  background: rgb(var(--c-elevated-rgb));
  color: rgb(var(--c-text-rgb));
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}
</style>
