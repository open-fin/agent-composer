<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{ yaml: string; downloadHref?: string; filename?: string }>()
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(props.yaml)
    copied.value = true
    setTimeout(() => (copied.value = false), 1800)
  } catch {
    // Clipboard access is blocked in some browsers; the YAML is on screen regardless.
    copied.value = false
  }
}
</script>

<template>
  <div class="card">
    <div class="row" style="justify-content: space-between; margin-bottom: 12px">
      <h3 style="margin: 0">Generated YAML</h3>
      <div class="row">
        <button type="button" @click="copy">{{ copied ? 'Copied' : 'Copy' }}</button>
        <a v-if="downloadHref" :href="downloadHref" :download="filename">
          <button type="button">Export YAML</button>
        </a>
      </div>
    </div>
    <pre class="yaml">{{ yaml }}</pre>
  </div>
</template>
