<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'
import { CopyLocal, GetHistoryData } from '../wailsjs/go/main/App'
import { useAppState, HistoryItem } from '../composables/useAppState'
import { useToast } from '../composables/useToast'

const state = useAppState()
const toast = useToast()

// 图片条目按 id 缓存渲染用 blob URL。
const imageUrls = ref<Record<string, string>>({})
// 拉取中的图片 id，避免并发触发重复拉取同一张图。
const loadingImages = new Set<string>()

async function ensureImageUrl(item: HistoryItem) {
  if (imageUrls.value[item.id]) return imageUrls.value[item.id]
  if (loadingImages.has(item.id)) return
  loadingImages.add(item.id)
  try {
    const b64 = (await GetHistoryData(item.id)) as string
    const binary = atob(b64)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
    const blob = new Blob([bytes], { type: 'image/png' })
    const url = URL.createObjectURL(blob)
    imageUrls.value[item.id] = url
    return url
  } finally {
    loadingImages.delete(item.id)
  }
}

// 对历史中的图片条目按需拉取：并发拉取缺失项，并回收已淘汰条目的 blob，避免长时运行内存累积。
watch(
  () => state.history.value.map((h) => h.id).join(','),
  async () => {
    const ids = new Set(state.history.value.map((h) => h.id))
    for (const id of Object.keys(imageUrls.value)) {
      if (!ids.has(id)) {
        URL.revokeObjectURL(imageUrls.value[id])
        delete imageUrls.value[id]
      }
    }
    const missing = state.history.value.filter(
      (h) => h.kind === 'image' && !imageUrls.value[h.id]
    )
    await Promise.all(missing.map((item) => ensureImageUrl(item)))
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  for (const url of Object.values(imageUrls.value)) {
    URL.revokeObjectURL(url)
  }
})

function fmtSize(n: number): string {
  if (n < 1024) return `${n} B`
  return `${(n / 1024).toFixed(1)} KB`
}

function fmtTime(s: string): string {
  const d = new Date(s)
  return d.toLocaleTimeString('zh-CN', { hour12: false })
}

async function onCopy(item: HistoryItem) {
  const err = await CopyLocal(item.id)
  if (err) {
    toast.err('复制失败：' + err)
    return
  }
  toast.ok('已复制并同步')
}
</script>

<template>
  <div class="pb-4">
    <header class="flex items-baseline justify-between mb-6">
      <div>
        <h1 class="text-[22px] font-semibold text-text leading-tight">历史记录</h1>
        <p class="text-sm text-muted mt-1">复制的内容将自动同步到已配对设备</p>
      </div>
      <span v-if="state.history.value.length" class="text-xs text-muted shrink-0">
        {{ state.history.value.length }} 条
      </span>
    </header>

    <!-- 空状态 -->
    <div v-if="state.history.value.length === 0" class="py-24 text-center">
      <div
        class="mx-auto w-12 h-12 rounded-2xl bg-white/[0.04] inline-flex items-center justify-center text-muted mb-4"
      >
        <Icon name="clipboard" :size="22" />
      </div>
      <p class="text-sm text-text">暂无记录</p>
      <p class="text-xs text-muted mt-1">在任意一端复制的内容将显示在此处</p>
    </div>

    <!-- 列表 -->
    <ul v-else>
      <li
        v-for="item in state.history.value"
        :key="item.id"
        class="group flex items-center gap-4 py-3.5"
      >
        <div
          class="shrink-0 w-12 h-12 rounded-xl bg-white/[0.04] flex items-center justify-center overflow-hidden text-muted"
        >
          <img
            v-if="item.kind === 'image' && imageUrls[item.id]"
            :src="imageUrls[item.id]"
            class="w-full h-full object-cover"
            :alt="item.id"
          />
          <Icon v-else :name="item.kind === 'image' ? 'image' : 'type'" :size="18" />
        </div>

        <div class="flex-1 min-w-0">
          <div
            class="flex items-center gap-2 text-[11px] text-muted whitespace-nowrap overflow-hidden"
          >
            <span>{{ item.kind === 'image' ? '图片' : '文本' }}</span>
            <span class="font-mono">{{ fmtSize(item.size) }}</span>
            <span class="opacity-40">·</span>
            <span class="truncate">来自 {{ item.from || '本机' }} · {{ fmtTime(item.time) }}</span>
          </div>
          <p
            v-if="item.kind === 'text'"
            class="text-sm text-text whitespace-pre-wrap break-words line-clamp-2 mt-1"
          >
            {{ item.preview }}
          </p>
        </div>

        <button
          class="btn btn-ghost h-8 px-3 shrink-0 opacity-0 group-hover:opacity-100 focus:opacity-100 transition"
          @click="onCopy(item)"
        >
          <Icon name="copy" :size="14" />
          复制
        </button>
      </li>
    </ul>
  </div>
</template>
