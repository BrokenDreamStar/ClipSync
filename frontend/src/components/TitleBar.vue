<script setup lang="ts">
// 仅 macOS 有自绘标题栏红绿灯：TitleBarHidden 模式下系统把红绿灯绘制在窗口顶部约 16pt、
// 距左缘约 8pt 处。标题栏做紧凑高度 32px，红绿灯正好落在 16pt 中线，标题居中即与红绿灯同轴。
// Windows/Linux 使用系统原生标题栏，不渲染此自绘标题栏，避免与系统标题栏重复。
const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
</script>

<template>
  <header v-if="isMac" class="drag-region shrink-0 bg-elevated pl-[68px]">
    <div class="h-[32px] px-5 flex items-center">
      <span class="text-text font-semibold text-[15px] tracking-wide leading-none select-none">ClipSync</span>
    </div>
  </header>
</template>
