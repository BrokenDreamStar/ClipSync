import { ref, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import {
  ApplyUpdate,
  CheckUpdate,
  GetAppVersion,
  GetUpdateState,
} from '../../bindings/clipsync/app'
import type { CheckResult, State } from '../../bindings/clipsync/internal/updater/models'
import { useToast } from './useToast'

// 软件更新（从 GitHub Releases）的前端状态，对应后端 internal/updater。
//
// 版本号以 Go 侧内嵌的 build/config.yml 为唯一事实来源（GetAppVersion 绑定），
// 前端不再硬编码 —— AGENTS.md 里「版本号三处同步」由此减为两处
//（build/config.yml + CHANGELOG.md）。
const version = ref('')
const check = ref<CheckResult | null>(null)
const checking = ref(false)
const state = ref<State | null>(null)

// 更新中 = 阶段不是 idle / error；期间禁用「关于」页的更新按钮。
const updating = computed(() => {
  const p = state.value?.phase
  return !!p && p !== 'idle' && p !== 'error'
})

// 下载字节数格式化（进度条旁展示）。
function fmtBytes(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n) || n <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

let eventsBound = false
function bindEvents() {
  if (eventsBound) return
  eventsBound = true
  // Go 侧 Emit 单个参数时 data 即对象本身（wails v3 EventManager.Emit 约定）。
  Events.On('updater:state', (ev) => {
    state.value = ev.data as State
  })
}

// initUpdate 在应用启动时调用一次：绑定进度事件、取版本号、静默检查更新。
// 静默检查不弹失败提示（断网 / 开发模式不该打扰托盘里的用户），
// 只在发现新版本时 toast 一次引导到「关于」页。
let initDone = false
async function initUpdate(): Promise<void> {
  if (initDone) return
  initDone = true
  bindEvents()
  try {
    // 界面上若有上次未完成的更新（窗口隐藏期间），先恢复进度展示。
    const [st] = await GetUpdateState()
    if (st && st.phase !== 'idle') state.value = st
    const [v, err] = await GetAppVersion()
    if (!err && v) version.value = v
  } catch {
    /* 预览 / 降级环境：保持空版本显示 */
  }
  void checkUpdate(true)
}

// checkUpdate 拉取一次检查结果（后端缓存 60 秒）。
// silent=true（启动自动检查）只在新版本可用时提示。
async function checkUpdate(silent = false): Promise<void> {
  if (checking.value || updating.value) return
  checking.value = true
  try {
    const [result] = await CheckUpdate()
    check.value = result
    if (silent && result.hasUpdate && result.latest) {
      useToast().push(`发现新版本 ${result.latest.tag}，可在「关于」页更新`, 'info', 6000)
    }
  } catch {
    check.value = null
    if (!silent) useToast().err('检查更新失败')
  } finally {
    checking.value = false
  }
}

// canApply：检查结果明确有更新、当前平台有安装包、且没有任务进行中。
const canApply = computed(
  () =>
    check.value?.hasUpdate === true &&
    !!check.value.latest?.assetName &&
    check.value.current.supported &&
    !checking.value &&
    !updating.value,
)

// applyUpdate 开始更新到指定 tag；进度走 updater:state 事件。
// 后台替换完成后应用自动退出，由 detached helper 拉起新版本。
async function applyUpdate(tag: string): Promise<void> {
  if (!canApply.value) return
  try {
    const [st, err] = await ApplyUpdate(tag)
    if (st) state.value = st
    if (err) {
      useToast().err(`更新失败：${err}`)
      const prev = state.value
      state.value = {
        phase: 'error',
        tag: prev?.tag ?? tag,
        received: prev?.received ?? 0,
        total: prev?.total ?? null,
        error: err,
      }
    }
  } catch {
    useToast().err('更新失败：无法调用后端更新服务')
  }
}

export function useUpdate() {
  return {
    version,
    check,
    checking,
    updating,
    canApply,
    state,
    fmtBytes,
    initUpdate,
    checkUpdate,
    applyUpdate,
  }
}
