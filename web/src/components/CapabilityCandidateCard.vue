<script setup lang="ts">
import type { CapabilityCandidate } from '../api/types'

defineProps<{ candidate: CapabilityCandidate; selected?: boolean }>()
defineEmits<{ (event: 'select', candidate: CapabilityCandidate): void }>()

/** Status colour follows meaning: terminal states are settled, needs_review is not. */
function statusClass(status: string): string {
  if (status === 'registered') return 'tag ok'
  if (status === 'rejected') return 'tag plain'
  if (status === 'needs_review') return 'tag warn'
  return 'tag'
}
</script>

<template>
  <button
    class="selectable"
    :class="{ selected }"
    type="button"
    @click="$emit('select', candidate)"
  >
    <div class="row" style="justify-content: space-between; align-items: flex-start">
      <div>
        <strong>{{ candidate.name }}</strong>
        <div class="small muted">{{ candidate.external_id }}</div>
      </div>
      <span :class="statusClass(candidate.status)">{{ candidate.status }}</span>
    </div>
    <div class="small muted" style="margin-top: 6px">
      {{ candidate.description || 'No description yet.' }}
    </div>
    <div class="row small" style="margin-top: 8px">
      <span class="tag plain">{{ candidate.candidate_type }}</span>
      <span class="muted">from {{ candidate.source_system }}</span>
    </div>
  </button>
</template>
