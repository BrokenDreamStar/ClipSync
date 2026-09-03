<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Icon from './Icon.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import { RemovePeer, ScanNow, IsAddrOnline, PairWith, PairDiscovered, RespondPairRequest } from '../wailsjs/go/main/App'
import { useAppState } from '../composables/useAppState'
import { useToast } from '../composables/useToast'

const state = useAppState()
const toast = useToast()

const peerOnline = ref<Record<string, boolean>>({})

async function refreshPairedFlags() {
  const next: Record<string, boolean> = {}
  for (const p of state.peers.value) {
    next[p.addr] = await IsAddrOnline(p.addr)
  }
  peerOnline.value = next
}

watch(() => state.peers.value, refreshPairedFlags, { immediate: true })

// 已配对判定：发现列表里某个设备是否已在本机对端列表（按 device_id 匹配）。
function isPaired(d: { device_id: string }): boolean {
  return state.peers.value.some((p) => p.id === d.device_id)
}

const sortedDiscovered = computed(() =>
  [...state.discovered.value].sort((a, b) => a.name.localeCompare(b.name))
)
const sortedPeers = computed(() => [...state.peers.value].sort((a, b) => a.name.localeCompare(b.name)))
const sortedPairRequests = computed(() => [...state.pairRequests.value].sort((a, b) => a.name.localeCompare(b.name)))

function ago(t: string): string {
  const diff = Date.now() - new Date(t).getTime()
  if (diff < 60_000) return `${Math.floor(diff / 1000)} 秒前`
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return `${Math.floor(diff / 86_400_000)} 天前`
}

async function onScan() {
  await ScanNow()
  toast.ok('已重新扫描')
}

async function onPair(d: { device_id: string; name: string }) {
  const peer = await PairDiscovered(d.device_id)
  if (!peer) {
    toast.err('配对失败')
    return
  }
  // PairDiscovered 返回 (PeerView, string)：空串为成功，否则为错误信息。
  if (typeof peer === 'string') {
    toast.err(peer)
    return
  }
  toast.ok('配对成功：' + peer.name)
  await state.refreshPeers()
}

const manualAddr = ref('')
const advancedOpen = ref(false)

async function onAddManual() {
  const addr = manualAddr.value.trim()
  if (!addr) return
  const res = await PairWith(addr)
  if (typeof res === 'string') {
    toast.err('配对失败：' + res)
    return
  }
  manualAddr.value = ''
  toast.ok('配对成功：' + res.name)
  await state.refreshPeers()
}

async function onAcceptRequest(d: { device_id: string; name: string }) {
  const err = await RespondPairRequest(d.device_id, true)
  if (err) {
    toast.err('同意失败：' + err)
    return
  }
  toast.ok('已同意来自 ' + d.name + ' 的配对')
  await state.refreshAll()
}

async function onRejectRequest(d: { device_id: string; name: string }) {
  const err = await RespondPairRequest(d.device_id, false)
  if (err) {
    toast.err('拒绝失败：' + err)
    return
  }
  toast.ok('已拒绝来自 ' + d.name + ' 的配对')
  await state.refreshPairRequests()
}

const pendingRemove = ref<string | null>(null)

function onRemove(addr: string) {
  pendingRemove.value = addr
}

function cancelRemove() {
  pendingRemove.value = null
}

async function confirmRemove() {
  const addr = pendingRemove.value
  if (!addr) return
  pendingRemove.value = null
  const err = await RemovePeer(addr)
  if (err) {
    toast.err('删除失败：' + err)
    return
  }
  toast.ok('已删除对端')
  await state.refreshPeers()
}
</script>

<template>
  <div class="pb-4">
    <header class="flex items-end justify-between gap-4 mb-8">
      <div>
        <h1 class="text-[22px] font-semibold text-text leading-tight">设备</h1>
        <p class="text-sm text-muted mt-1">扫描局域网内的其他 ClipSync 设备并配对</p>
      </div>
      <button class="btn btn-ghost h-8 px-3 shrink-0" @click="onScan">
        <Icon name="refresh" :size="15" />
        立即扫描
      </button>
    </header>

    <!-- 入站配对请求 -->
    <section v-if="sortedPairRequests.length > 0" class="mb-10">
      <div class="flex items-baseline justify-between mb-1.5">
        <h2 class="text-[15px] font-semibold text-text">配对请求</h2>
        <span class="text-xs text-muted">{{ sortedPairRequests.length }} 条待确认</span>
      </div>
      <ul>
        <li
          v-for="r in sortedPairRequests"
          :key="r.device_id"
          class="flex items-center gap-3 py-3 border-t border-hairline"
        >
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2.5">
              <span class="text-sm font-medium text-text">{{ r.name }}</span>
              <span class="text-xs text-muted font-mono">{{ r.addr }}</span>
            </div>
            <div class="text-xs text-muted mt-0.5">请求与本机同步剪贴板</div>
          </div>
          <button class="btn btn-primary h-8 px-3.5 shrink-0" @click="onAcceptRequest(r)">同意</button>
          <button class="btn btn-ghost h-8 px-3.5 shrink-0" @click="onRejectRequest(r)">拒绝</button>
        </li>
      </ul>
    </section>

    <!-- 发现的设备 -->
    <section class="mb-10">
      <div class="flex items-baseline justify-between mb-1.5">
        <h2 class="text-[15px] font-semibold text-text">发现的设备</h2>
        <span class="text-xs text-muted">{{ sortedDiscovered.length }} 台在线</span>
      </div>

      <div v-if="sortedDiscovered.length === 0" class="py-8 text-center">
        <div
          class="mx-auto w-10 h-10 rounded-xl bg-overlay inline-flex items-center justify-center text-muted mb-3"
        >
          <Icon name="radar" :size="18" />
        </div>
        <p class="text-sm text-text">未发现设备</p>
      </div>

      <ul v-else>
        <li v-for="d in sortedDiscovered" :key="d.device_id" class="flex items-center gap-3 py-3">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2.5">
              <span class="text-sm font-medium text-text">{{ d.name }}</span>
              <span class="text-xs text-muted font-mono">{{ d.addr }}</span>
            </div>
            <div class="text-xs text-muted mt-0.5">上次发现 {{ ago(d.last_seen) }}</div>
          </div>
          <button
            v-if="isPaired(d)"
            class="flex items-center gap-1.5 text-xs text-success shrink-0 cursor-default"
          >
            <Icon name="check-circle" :size="14" />
            已配对
          </button>
          <button v-else class="btn btn-primary h-8 px-3.5 shrink-0" @click="onPair(d)">配对</button>
        </li>
      </ul>
    </section>

    <!-- 已配对设备 -->
    <section class="mb-10">
      <div class="flex items-baseline justify-between mb-1.5">
        <h2 class="text-[15px] font-semibold text-text">已配对设备</h2>
      </div>

      <div v-if="sortedPeers.length === 0" class="py-16 text-center">
        <div
          class="mx-auto w-12 h-12 rounded-2xl bg-overlay inline-flex items-center justify-center text-muted mb-4"
        >
          <Icon name="link" :size="22" />
        </div>
        <p class="text-sm text-text">尚未配对设备</p>
        <p class="text-xs text-muted mt-1">在上方发现列表中点击「配对」，对方确认后即建立连接</p>
      </div>

      <ul v-else>
        <li
          v-for="p in sortedPeers"
          :key="p.id"
          class="flex items-center justify-between gap-3 py-3"
        >
          <div class="flex items-center gap-3 min-w-0">
            <div
              class="w-10 h-10 rounded-xl bg-overlay inline-flex items-center justify-center text-muted shrink-0"
            >
              <Icon name="monitor" :size="17" />
            </div>
            <div class="min-w-0">
              <div class="text-sm font-medium text-text truncate">{{ p.name }}</div>
              <div class="text-xs text-muted font-mono truncate">{{ p.addr }}</div>
              <div class="flex items-center gap-1.5 text-[11px] text-muted mt-0.5">
                <span
                  class="w-1.5 h-1.5 rounded-full shrink-0"
                  :class="peerOnline[p.addr] ? 'bg-success' : 'bg-muted/60'"
                />
                {{ peerOnline[p.addr] ? '在线' : '离线' }}
              </div>
            </div>
          </div>
          <button class="btn-icon shrink-0" :title="'删除 ' + p.name" @click="onRemove(p.addr)">
            <Icon name="trash" :size="15" />
          </button>
        </li>
      </ul>
    </section>

    <!-- 手动配对 -->
    <section>
      <button
        class="text-xs text-muted hover:text-text transition flex items-center gap-1.5 cursor-pointer"
        @click="advancedOpen = !advancedOpen"
      >
        <span class="inline-block transition" :class="advancedOpen ? 'rotate-90' : ''">
          <Icon name="chevron-right" :size="14" />
        </span>
        手动配对 IP / 端口
      </button>
      <div v-if="advancedOpen" class="mt-3 flex gap-2">
        <input
          v-model="manualAddr"
          class="input flex-1 font-mono"
          placeholder="host:port，如 192.168.1.20:9250"
          @keyup.enter="onAddManual"
        />
        <button class="btn btn-primary" @click="onAddManual">
          <Icon name="plus" :size="16" />
          发起配对
        </button>
      </div>
    </section>

    <ConfirmDialog
      :open="!!pendingRemove"
      title="删除已配对设备"
      :message="pendingRemove ? `确定删除该设备？删除后将断开连接。` : ''"
      confirm-text="删除"
      cancel-text="取消"
      danger
      @confirm="confirmRemove"
      @cancel="cancelRemove"
    />
  </div>
</template>
