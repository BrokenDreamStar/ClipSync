<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import Icon from './Icon.vue'
import { SaveConfig } from '../wailsjs/go/main/App'
import { useAppState } from '../composables/useAppState'
import { useToast } from '../composables/useToast'

const state = useAppState()
const toast = useToast()

const name = ref('')
const port = ref(9250)

watch(
  () => state.cfg.value,
  (c) => {
    if (!c) return
    name.value = c.name
    port.value = c.port
  },
  { immediate: true }
)

const deviceID = computed(() => state.cfg.value?.device_id ?? '')

async function onSave() {
  const err = await SaveConfig(name.value, Number(port.value))
  if (err) {
    toast.err('保存失败：' + err)
    return
  }
  toast.ok('配置已保存。修改本机名或端口需重启生效。')
}
</script>

<template>
  <div class="pb-4">
    <header class="mb-10">
      <h1 class="text-[22px] font-semibold text-text leading-tight">设置</h1>
      <p class="text-sm text-muted mt-1">本机信息与同步参数</p>
    </header>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-5">本机信息</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <label class="block">
          <span class="field-label">本机名</span>
          <input v-model="name" class="input" placeholder="如 my-mac" />
        </label>
        <label class="block">
          <span class="field-label">端口</span>
          <input v-model.number="port" type="number" class="input font-mono" />
        </label>
      </div>
      <p class="text-xs text-muted mt-4 break-all">
        设备 ID：<span class="font-mono">{{ deviceID }}</span>
      </p>
    </section>

    <section class="mb-10">
      <h2 class="text-[15px] font-semibold text-text mb-3">配对方式</h2>
      <p class="text-sm text-muted leading-relaxed">
        在「设备」页扫描到局域网内的 ClipSync 设备后，点击「配对」发起请求；对方同意后即完成配对并自动同步。无需在两台机器上配置相同的令牌。
      </p>
    </section>

    <div class="flex items-center justify-between gap-4">
      <p class="text-xs text-muted">修改本机名或端口需重启生效</p>
      <button class="btn btn-primary" @click="onSave">
        <Icon name="check" :size="16" />
        保存配置
      </button>
    </div>
  </div>
</template>
