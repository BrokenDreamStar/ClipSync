<script setup lang="ts">
import Icon from './Icon.vue'
import appIcon from '../assets/appicon.png'

// 版本号需与 wails.json 中的 productVersion 保持一致
const version = '0.0.1'

const highlights = [
  { icon: 'zap', text: '局域网内剪贴板实时同步' },
  { icon: 'shield', text: '配对加密传输，数据不出内网' },
  { icon: 'wifi', text: '自动发现同网段设备，无需手动配置' },
]
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
      <p class="text-xs text-muted mt-1 font-mono">v{{ version }}</p>
      <p class="text-sm text-muted mt-4 leading-relaxed max-w-sm">
        一款轻量的局域网剪贴板同步工具，在多台设备间即时共享文本与图片。
      </p>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">特性</h2>
      <ul class="space-y-3">
        <li v-for="h in highlights" :key="h.text" class="flex items-center gap-3 text-sm text-muted">
          <span
            class="w-7 h-7 rounded-lg bg-white/[0.05] inline-flex items-center justify-center text-accent shrink-0"
          >
            <Icon :name="h.icon" :size="15" />
          </span>
          {{ h.text }}
        </li>
      </ul>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">技术信息</h2>
      <div class="rounded-xl bg-white/[0.03] px-4 py-3 space-y-2 text-sm">
        <div class="flex justify-between gap-4">
          <span class="text-muted">运行时</span>
          <span class="text-text">Go + Wails v2</span>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">界面</span>
          <span class="text-text">Vue 3 + Tailwind CSS</span>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">网络</span>
          <span class="text-text font-mono text-xs leading-5">mDNS 发现 · TCP 同步</span>
        </div>
      </div>
    </section>

    <p class="text-center text-xs text-muted">仅供学习与个人使用 · © 2026 ClipSync</p>
  </div>
</template>
