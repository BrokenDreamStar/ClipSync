<script setup lang="ts">
import Icon from './Icon.vue'
import appIcon from '../assets/appicon.png'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'

// 版本号需与 wails.json 中的 productVersion 保持一致
const version = '0.0.2'
const author = 'BrokenDream_Star'
const repoURL = 'https://github.com/BrokenDreamStar/ClipSync'
const licenseURL = 'https://www.gnu.org/licenses/lgpl-3.0.html'

const highlights = [
  { icon: 'zap', text: '局域网内自动同步剪贴板' },
  { icon: 'shield', text: '数据仅在本局域网内传输' },
  { icon: 'wifi', text: '自动发现同网段设备，配对一次即可' },
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
        一款轻量级的跨平台剪贴板同步工具
      </p>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">特性</h2>
      <ul class="space-y-3">
        <li v-for="h in highlights" :key="h.text" class="flex items-center gap-3 text-sm text-muted">
          <span
            class="w-7 h-7 rounded-lg bg-overlay inline-flex items-center justify-center text-accent shrink-0"
          >
            <Icon :name="h.icon" :size="15" />
          </span>
          {{ h.text }}
        </li>
      </ul>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-4">技术信息</h2>
      <div class="rounded-xl bg-overlay-soft px-4 py-3 space-y-2 text-sm">
        <div class="flex justify-between gap-4">
          <span class="text-muted">运行时</span>
          <span class="text-text">Go + Wails v2</span>
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
            @click="BrowserOpenURL(repoURL)"
          >
            <Icon name="link" :size="14" />
            BrokenDreamStar/ClipSync
          </button>
        </div>
        <div class="flex justify-between gap-4">
          <span class="text-muted">许可证</span>
          <button
            class="inline-flex items-center gap-1.5 text-accent hover:text-accent-hover cursor-pointer"
            @click="BrowserOpenURL(licenseURL)"
          >
            <Icon name="shield" :size="14" />
            LGPL v3
          </button>
        </div>
      </div>
    </section>

    <p class="text-center text-xs text-muted">© 2021 - 2026 StarM Team · {{ author }}</p>
  </div>
</template>
