<script setup lang="ts">
// 通用开关（switch）。v-model 绑定布尔值；状态变更通过 update:modelValue 事件上抛。
const props = withDefaults(defineProps<{ modelValue: boolean; disabled?: boolean }>(), {
  disabled: false,
})
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>

<template>
  <button type="button" role="switch" :aria-checked="modelValue" class="switch" :class="{ on: modelValue }" @click="toggle">
    <span class="knob" />
  </button>
</template>

<style scoped>
.switch {
  position: relative;
  width: 40px;
  height: 22px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: rgb(var(--c-elevated-rgb) / 0.9);
  cursor: pointer;
  transition: background 0.18s ease, border-color 0.18s ease;
  flex: none;
  padding: 0;
}
.switch.on {
  background: #14b8a6;
  border-color: transparent;
}
.knob {
  position: absolute;
  top: 50%;
  left: 3px;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  transition: left 0.18s ease;
}
.switch.on .knob {
  left: 19px;
}
</style>
