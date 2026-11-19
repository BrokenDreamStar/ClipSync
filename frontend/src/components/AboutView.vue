<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import appIcon from '../assets/appicon.png'
import ConfirmDialog from './ConfirmDialog.vue'
import { Browser } from '@wailsio/runtime'
import { useUpdate } from '../composables/useUpdate'
import type { State } from '../../bindings/clipsync/internal/updater/models'

// 版本号不再硬编码：以 GetAppVersion 绑定（Go 侧内嵌 build/config.yml）为准，
// 与 Info.plist / Windows syso 同源。
const author = 'BrokenDream_Star'
const repoURL = 'https://github.com/BrokenDreamStar/ClipSync'
const licenseURL = 'https://www.gnu.org/licenses/lgpl-3.0.html'

const { version, check, checking, updating, canApply, state, fmtBytes, checkUpdate, applyUpdate } =
  useUpdate()

const showConfirm = ref(false)

// 打开「关于」时若无检查结果就补一次（启动时已静默检查过则直接展示）。
onMounted(() => {
  if (!check.value) void checkUpdate()
})

const statusText = computed(() => {
  const c = check.value
  if (!c) return checking.value ? '正在检查更新…' : '尚未检查更新'
  if (!c.current.supported) return c.error || '当前运行方式不支持自更新'
  if (c.error) return c.error
  if (c.hasUpdate && c.latest) return `有可用更新：${c.latest.tag}（当前 v${c.current.version}）`
  if (c.latest) return `已是最新版本（${c.latest.tag}）`
  return '未获取到最新版本信息'
})

// 进度条：只展示「有任务进行中」或刚失败的状态。
const progressVisible = computed(() => {
  const p = state.value?.phase
  return !!p && p !== 'idle'
})

const progressPercent = computed(() => {
  const s = state.value
  if (!s?.total || s.total <= 0) return 0
  return Math.min(100, Math.round((s.received / s.total) * 100))
})

const progressFailed = computed(() => state.value?.phase === 'error')

const phaseText = computed(() => {
  const s = state.value as State | null
  if (!s) return ''
  switch (s.phase) {
    case 'downloading':
      return `正在下载 ${progressPercent.value}%`
    case 'verifying':
      return '正在校验安装包…'
    case 'installing':
      return '正在安装新版本…'
    case 'restarting':
      return '安装完成，应用即将自动重启…'
    case 'error':
      return s.error ? `更新失败：${s.error}` : '更新失败'
    default:
      return ''
  }
})

const bytesText = computed(() => {
  const s = state.value
  if (!s || s.phase !== 'downloading') return ''
  return `${fmtBytes(s.received)} / ${fmtBytes(s.total)}`
})

function onConfirmUpdate() {
  showConfirm.value = false
  const tag = check.value?.latest?.tag
  if (tag) void applyUpdate(tag)
}

// 检查与更新合并为一个按钮：有可用更新时走更新确认，否则重新检查。
const actionLabel = computed(() => {
  if (updating.value) return '更新中…'
  if (checking.value) return '检查中…'
  if (canApply.value) return '立即更新'
  return '检查更新'
})

function onAction() {
  if (checking.value || updating.value) return
  if (canApply.value) showConfirm.value = true
  else void checkUpdate()
}
</script>

<template>
  <div class="pb-4">
    <header class="mb-10">
      <h1 class="text-[22px] font-semibold text-text leading-tight">关于</h1>
      <p class="text-sm text-muted mt-1">应用信息与版本</p>
    </header>

    <section class="mb-10 flex flex-col items-center text-center pt-4">
      <img :src="appIcon" alt="ClipSync 图标" class="w-14 h-14 rounded-2xl mb-4 shadow-lg" />
      <h2 class="text-lg font-semibold text-text leading-tight">ClipSync</h2>
      <p class="text-xs text-muted mt-1 font-mono">v{{ version || '—' }}</p>
      <p class="text-sm text-muted mt-4 leading-relaxed max-w-sm">
        一款轻量级的跨平台剪贴板同步工具
      </p>
    </section>

    <!-- 软件更新：单个按钮（检查/更新随状态切换），下载中展示进度条 -->
    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">软件更新</h2>
      <div class="rounded-xl bg-overlay-soft px-4 py-3 space-y-3 text-sm">
        <div class="flex items-center justify-between gap-4 flex-wrap">
          <span class="text-muted">
            版本
            <span class="text-text font-mono ml-1">v{{ version || '—' }}</span>
          </span>
          <button
            class="btn"
            :class="canApply ? 'btn-primary' : 'btn-ghost'"
            :disabled="checking || updating"
            @click="onAction"
          >
            {{ actionLabel }}
          </button>
        </div>

        <div class="flex items-center gap-3 flex-wrap">
          <p class="text-muted" :class="{ 'text-accent': canApply }">{{ statusText }}</p>
          <button
            v-if="check?.latest?.htmlUrl"
            class="inline-flex items-center gap-1.5 text-accent hover:text-accent-hover shrink-0"
            @click="Browser.OpenURL(check.latest.htmlUrl)"
          >
            <Icon name="link" :size="14" />
            发布说明
          </button>
        </div>

        <div v-if="progressVisible">
          <div class="h-1.5 rounded-full bg-line overflow-hidden">
            <div
              class="h-full rounded-full transition-[width] duration-300"
              :class="progressFailed ? 'bg-danger' : 'bg-accent'"
              :style="{ width: progressFailed ? '100%' : `${progressPercent}%` }"
            />
          </div>
          <div class="flex items-center justify-between gap-4 mt-1.5 text-xs">
            <span :class="progressFailed ? 'text-danger' : 'text-muted'" class="break-all">
              {{ phaseText }}
            </span>
            <span v-if="bytesText" class="text-muted shrink-0 font-mono">{{ bytesText }}</span>
          </div>
        </div>
      </div>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">技术信息</h2>
      <div class="rounded-xl bg-overlay-soft px-4 py-3 space-y-2 text-sm">
        <div class="flex justify-between gap-4">
          <span class="text-muted">运行时</span>
          <span class="text-text">Go + Wails v3</span>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">界面</span>
          <span class="text-text">Vue 3 + Tailwind CSS</span>
        </div>
      </div>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">作者</h2>
      <div class="rounded-xl bg-overlay-soft px-4 py-3 space-y-2 text-sm">
        <div class="flex justify-between gap-4">
          <span class="text-muted">作者</span>
          <span class="text-text">{{ author }}</span>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">GitHub</span>
          <button
            class="inline-flex items-center gap-1.5 text-accent hover:text-accent-hover cursor-pointer"
            @click="Browser.OpenURL(repoURL)"
          >
            <Icon name="link" :size="14" />
            BrokenDreamStar/ClipSync
          </button>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">许可证</span>
          <button
            class="inline-flex items-center gap-1.5 text-accent hover:text-accent-hover cursor-pointer"
            @click="Browser.OpenURL(licenseURL)"
          >
            <Icon name="shield" :size="14" />
            LGPL v3
          </button>
        </div>
      </div>
    </section>

    <p class="text-center text-xs text-muted">© 2021 - 2026 StarM Team · {{ author }}</p>

    <ConfirmDialog
      :open="showConfirm"
      title="更新 ClipSync"
      :message="`确定更新到 ${check?.latest?.tag ?? ''}？下载并校验完成后应用会自动重启。`"
      confirm-text="更新"
      cancel-text="取消"
      @confirm="onConfirmUpdate"
      @cancel="showConfirm = false"
    />
  </div>
</template>
