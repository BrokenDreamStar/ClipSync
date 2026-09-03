import { ref, computed, watch } from 'vue'
import { SetTheme } from '../wailsjs/go/main/App'
import { useAppState } from './useAppState'

export type ThemePref = 'dark' | 'light' | 'system'

export interface ThemeOption {
  value: ThemePref
  label: string
  icon: string
}

export const themeOptions: ThemeOption[] = [
  { value: 'light', label: '浅色', icon: 'sun' },
  { value: 'dark', label: '深色', icon: 'moon' },
  { value: 'system', label: '跟随系统', icon: 'monitor' },
]

const THEME_STORAGE_KEY = 'clipsync-theme'

// ---- 模块级共享状态（与 useToast 同模式）----

// 系统是否处于深色模式；matchMedia 不可用时按深色兜底。
const systemDark = ref(true)
const pref = ref<ThemePref>('dark')

// 主题偏好解析后的实际深色状态（"system" 随 matchMedia 实时变化）。
const resolvedDark = computed(() =>
  pref.value === 'system' ? systemDark.value : pref.value === 'dark'
)

if (typeof window !== 'undefined' && window.matchMedia) {
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  systemDark.value = mql.matches
  mql.addEventListener('change', (e) => {
    systemDark.value = e.matches
  })
}

// 把解析结果落到 <html> 的 class 上；同时记录偏好，供 index.html 内联脚本
// 在下一次启动的首帧抢先应用（消除浅色主题下的深色闪烁）。
function applyToDOM() {
  const el = document.documentElement
  el.classList.toggle('dark', resolvedDark.value)
  el.classList.toggle('light', !resolvedDark.value)
  try {
    localStorage.setItem(THEME_STORAGE_KEY, pref.value)
  } catch {
    /* localStorage 不可用时忽略，仅影响下次启动首帧 */
  }
}

watch(resolvedDark, applyToDOM, { immediate: true })

// 乐观切换主题：立即应用，持久化失败时回滚。返回错误信息（空串为成功）。
export async function setThemePref(t: ThemePref): Promise<string> {
  const prev = pref.value
  if (t === prev) return ''
  pref.value = t
  const err = await SetTheme(t)
  if (err) {
    pref.value = prev
  }
  return err ?? ''
}

// 当前主题偏好（乐观更新；后端配置事件到达后与之收敛）。
export const themePref = computed<ThemePref>(() => pref.value)

// useTheme 在组件 setup 中调用一次（App.vue）：以后端配置为唯一事实来源，
// 同步主题偏好；后端变更（如事件刷新）会自动反映到界面。
export function useTheme() {
  const state = useAppState()

  watch(
    () => state.cfg.value?.theme,
    (t) => {
      if (t === 'dark' || t === 'light' || t === 'system') {
        pref.value = t
      }
    },
    { immediate: true }
  )
}
