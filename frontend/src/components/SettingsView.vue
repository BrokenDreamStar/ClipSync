<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { SaveConfig, GetAutostart, SetAutostart, SetStartMinimized } from '../../bindings/clipsync/app'
import { useAppState } from '../composables/useAppState'
import { useToast } from '../composables/useToast'
import { themeOptions, setThemePref, themePref, type ThemePref } from '../composables/useTheme'

const state = useAppState()
const toast = useToast()

const name = ref('')
const port = ref(9250)
// 声明必须在下方 immediate watch 之前：首次进入设置页时 cfg 已由其他视图填充，
// 回调会立即执行并写入该 ref。
const startMinimized = ref(false)

watch(
  () => state.cfg.value,
  (c) => {
    if (!c) return
    name.value = c.name
    port.value = c.port
    startMinimized.value = !!c.start_minimized
  },
  { immediate: true }
)

const deviceID = computed(() => state.cfg.value?.device_id ?? '')

const valid = computed(() => {
  const p = Number(port.value)
  return name.value.trim() !== '' && Number.isInteger(p) && p >= 1 && p <= 65535
})

// 修改后自动保存：输入停顿 600ms 再落盘，避免每敲一个字符写一次配置。
// 与当前配置相同（含首次从配置回填）时不重复保存。
const saved = ref(false)
let saveTimer: ReturnType<typeof setTimeout> | undefined
let savedTimer: ReturnType<typeof setTimeout> | undefined

async function autoSave() {
  if (!valid.value) return
  const n = name.value.trim()
  const p = Number(port.value)
  const cfg = state.cfg.value
  if (!cfg || (n === cfg.name && p === cfg.port)) return
  const err = await SaveConfig(n, p)
  if (err) {
    toast.err('自动保存失败：' + err)
    return
  }
  saved.value = true
  clearTimeout(savedTimer)
  savedTimer = setTimeout(() => (saved.value = false), 2000)
}

watch([name, port], () => {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => void autoSave(), 600)
})

onBeforeUnmount(() => {
  // 离开页面时立刻补存最后一次未落盘的输入。
  clearTimeout(saveTimer)
  void autoSave()
  clearTimeout(savedTimer)
})

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

// ---- 启动时最小化到任务栏 ----

async function onStartMinimized(enable: boolean) {
  const err = await SetStartMinimized(enable)
  if (err) {
    // 保存失败：回滚开关状态并提示。
    startMinimized.value = !enable
    toast.err('设置启动时最小化失败：' + err)
    return
  }
  toast.ok(enable ? '下次启动时仅在托盘显示图标' : '下次启动时将显示主窗口')
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
      <div class="flex items-center gap-3 mt-2">
        <p class="text-xs" :class="valid ? 'text-muted' : 'text-danger'">
          {{ valid ? '修改后自动保存，重启应用生效' : '本机名不能为空，端口需为 1–65535' }}
        </p>
        <span v-if="saved" class="text-xs text-success shrink-0">已自动保存</span>
      </div>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">外观</h2>
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
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">通用</h2>
      <div class="space-y-5">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm text-text">开机自启</p>
            <p class="text-xs text-muted mt-1">登录系统后自动在后台启动 ClipSync</p>
          </div>
          <ToggleSwitch v-model="autostart" :disabled="autostartLoading" @update:model-value="onAutostart" />
        </div>
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm text-text">启动时最小化到任务栏</p>
            <p class="text-xs text-muted mt-1">启动时不弹出主窗口，仅在托盘显示图标</p>
          </div>
          <ToggleSwitch v-model="startMinimized" @update:model-value="onStartMinimized" />
        </div>
      </div>
    </section>
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
