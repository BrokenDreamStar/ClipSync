import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import {
  GetConfig,
  GetPeers,
  GetDiscovered,
  GetConnectedPeers,
  GetHistory,
  GetPairRequests,
} from '../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'

export interface HistoryItem {
  id: string
  kind: string
  from: string
  size: number
  time: string
  preview: string
}

export interface DiscoveredPeer {
  device_id: string
  name: string
  addr: string
  last_seen: string
}

export interface PeerView {
  id: string
  name: string
  addr: string
}

export interface PairRequest {
  device_id: string
  name: string
  addr: string
  time: string
}

export interface AppConfig {
  name: string
  port: number
  device_id: string
  peers: PeerView[]
}

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
  const [c, p, d, cp, h, pr] = await Promise.all([
    GetConfig(),
    GetPeers(),
    GetDiscovered(),
    GetConnectedPeers(),
    GetHistory(),
    GetPairRequests(),
  ])
  cfg.value = (c as AppConfig) ?? null
  peers.value = (p as PeerView[]) ?? []
  discovered.value = (d as DiscoveredPeer[]) ?? []
  connected.value = (cp as string[]) ?? []
  history.value = (h as HistoryItem[]) ?? []
  pairRequests.value = (pr as PairRequest[]) ?? []
}

async function refreshHistory() {
  const r = await GetHistory()
  history.value = (r as HistoryItem[]) ?? []
}

async function refreshPeers() {
  peers.value = await GetPeers()
  connected.value = await GetConnectedPeers()
}

async function refreshDiscovered() {
  discovered.value = await GetDiscovered()
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
    EventsOn('engine:history', refreshHistoryDebounced)
    EventsOn('engine:peers', refreshPeersDebounced)
    EventsOn('engine:discovered', refreshDiscoveredDebounced)
    EventsOn('engine:config', refreshAll)
    EventsOn('engine:pair_request', refreshPairRequests)
  })

  onBeforeUnmount(() => {
    EventsOff('engine:history')
    EventsOff('engine:peers')
    EventsOff('engine:discovered')
    EventsOff('engine:config')
    EventsOff('engine:pair_request')
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
