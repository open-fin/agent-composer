<script setup lang="ts">
import type { Capability } from '../api/types'

defineProps<{ capability: Capability; selected?: boolean }>()
defineEmits<{ (event: 'select', capability: Capability): void }>()

function riskClass(risk: string): string {
  if (risk === 'high') return 'tag danger'
  if (risk === 'medium') return 'tag warn'
  return 'tag plain'
}
</script>

<template>
  <button
    class="selectable"
    :class="{ selected }"
    type="button"
    @click="$emit('select', capability)"
  >
    <div class="row" style="justify-content: space-between; align-items: flex-start">
      <div>
        <strong>{{ capability.name }}</strong>
        <!-- The slug is what composition YAML references, so it is always visible. -->
        <div class="small muted mono">{{ capability.slug }}</div>
      </div>
      <span :class="riskClass(capability.risk_level)">{{ capability.risk_level }}</span>
    </div>
    <div class="small muted" style="margin-top: 6px">{{ capability.description }}</div>
    <div class="row small" style="margin-top: 8px">
      <span class="tag plain">{{ capability.type }}</span>
      <span v-for="intent in capability.intents" :key="intent" class="tag">{{ intent }}</span>
    </div>
  </button>
</template>
