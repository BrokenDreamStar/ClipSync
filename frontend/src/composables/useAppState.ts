import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import {
  GetConfig,
  GetPeers,
  GetDiscovered,
  GetConnectedPeers,
  GetHistory,
  GetPairRequests,
} from '../../bindings/clipsync/app'
import { Events } from '@wailsio/runtime'
import type { ConfigView, HistoryView, PeerView } from '../../bindings/clipsync/models'
import type {
  DiscoveredPeer,
  PairRequest,
} from '../../bindings/clipsync/internal/engine/models'

// 数据形状以生成的 models 为唯一事实来源；沿用旧别名保持下游导入不变。
export type HistoryItem = HistoryView
export type AppConfig = ConfigView
export type { PeerView, DiscoveredPeer, PairRequest }

const cfg = ref<AppConfig | null>(null)
const peers = ref<PeerView[]>([])
const discovered = ref<DiscoveredPeer[]>([])
const connected = ref<string[]>([])
const history = ref<HistoryItem[]>([])
const pairRequests = ref<PairRequest[]>([])

const onlineLabel = computed(() => {
  if (connected.value.length === 0) return '未连接'
  return '已连接'
})

// debounce 把高频触发的事件（如 engine:history 每次复制都发）合并成一次拉取/重渲染，
// 避免连续事件风暴带来整表刷新与界面卡顿。
function debounce(fn: () => void, wait: number) {
  let timer: ReturnType<typeof setTimeout> | undefined
  return () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(fn, wait)
  }
}

async function refreshAll() {
  // v3 绑定下，(T, string) 双返回值方法 resolve 为元组；第二项空串为成功。
  const [cfgV, p, d, cp, histV, pr] = await Promise.all([
    GetConfig(),
    GetPeers(),
    GetDiscovered(),
    GetConnectedPeers(),
    GetHistory(),
    GetPairRequests(),
  ])
  cfg.value = cfgV[0]
  peers.value = p ?? []
  discovered.value = d ?? []
  connected.value = cp ?? []
  history.value = histV[0] ?? []
  pairRequests.value = pr ?? []
}

async function refreshHistory() {
  const [items] = await GetHistory()
  history.value = items ?? []
}

async function refreshPeers() {
  peers.value = (await GetPeers()) ?? []
  connected.value = (await GetConnectedPeers()) ?? []
}

async function refreshDiscovered() {
  discovered.value = (await GetDiscovered()) ?? []
}

async function refreshPairRequests() {
  pairRequests.value = (await GetPairRequests()) ?? []
}

const refreshHistoryDebounced = debounce(refreshHistory, 150)
const refreshPeersDebounced = debounce(refreshPeers, 150)
const refreshDiscoveredDebounced = debounce(refreshDiscovered, 150)

export function useAppState() {
  onMounted(async () => {
    await refreshAll()
    Events.On('engine:history', refreshHistoryDebounced)
    Events.On('engine:peers', refreshPeersDebounced)
    Events.On('engine:discovered', refreshDiscoveredDebounced)
    Events.On('engine:config', refreshAll)
    Events.On('engine:pair_request', refreshPairRequests)
  })

  onBeforeUnmount(() => {
    Events.Off('engine:history')
    Events.Off('engine:peers')
    Events.Off('engine:discovered')
    Events.Off('engine:config')
    Events.Off('engine:pair_request')
  })

  return {
    cfg,
    peers,
    discovered,
    connected,
    history,
    pairRequests,
    onlineLabel,
    refreshAll,
    refreshHistory,
    refreshPeers,
    refreshDiscovered,
    refreshPairRequests,
  }
}
