<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { message } from '../api/client'
import { listCandidates } from '../api/candidates'
import CapabilityCandidateCard from '../components/CapabilityCandidateCard.vue'
import CapabilityReviewPanel from '../components/CapabilityReviewPanel.vue'
import type { CapabilityCandidate } from '../api/types'

const CANDIDATE_STATUSES = ['', 'extracted', 'inferred', 'needs_review', 'registered', 'rejected']

const candidates = ref<CapabilityCandidate[]>([])
const selected = ref<CapabilityCandidate | null>(null)
const statusFilter = ref('')
const search = ref('')
const loading = ref(false)
const error = ref('')

async function load(keepSelectionID?: string) {
  loading.value = true
  error.value = ''
  try {
    const page = await listCandidates({ status: statusFilter.value, q: search.value, limit: 200 })
    candidates.value = page.items
    const wanted = keepSelectionID ?? selected.value?.id
    selected.value = candidates.value.find((candidate) => candidate.id === wanted) ?? candidates.value[0] ?? null
  } catch (caught) {
    error.value = message(caught)
  } finally {
    loading.value = false
  }
}

onMounted(() => load())
watch([statusFilter, search], () => load())

const pendingCount = computed(
  () => candidates.value.filter((candidate) => ['extracted', 'inferred', 'needs_review'].includes(candidate.status)).length,
)

function onChanged(candidate: CapabilityCandidate) {
  load(candidate.id)
}

function onRegistered(candidateID: string) {
  load(candidateID)
}
</script>

<template>
  <h2 class="page-title">Extracted Capabilities</h2>
  <p class="page-subtitle">
    Candidates waiting to enter the registry. Fields the system inferred are called out in the
    review panel, so you can check the guesses before accepting them.
  </p>

  <div class="card" style="margin-bottom: 16px">
    <div class="row">
      <button
        v-for="status in CANDIDATE_STATUSES"
        :key="status || 'all'"
        type="button"
        class="chip"
        :class="{ active: statusFilter === status }"
        @click="statusFilter = status"
      >
        {{ status || 'all' }}
      </button>
      <span class="spacer" style="flex: 1"></span>
      <input v-model="search" type="text" placeholder="Search name or description" style="max-width: 280px" />
    </div>
    <div class="small muted" style="margin-top: 8px">
      {{ candidates.length }} shown · {{ pendingCount }} awaiting a decision
    </div>
  </div>

  <div v-if="error" class="banner error" style="margin-bottom: 16px">{{ error }}</div>

  <div class="split">
    <div>
      <div v-if="loading && !candidates.length" class="card empty">Loading…</div>
      <div v-else-if="!candidates.length" class="card empty">
        No candidates yet. Import a Dify DSL on the Source Import page.
      </div>
      <div v-else class="list">
        <CapabilityCandidateCard
          v-for="candidate in candidates"
          :key="candidate.id"
          :candidate="candidate"
          :selected="candidate.id === selected?.id"
          @select="selected = $event"
        />
      </div>
    </div>

    <CapabilityReviewPanel
      v-if="selected"
      :candidate="selected"
      @changed="onChanged"
      @registered="onRegistered"
    />
    <div v-else class="card empty">Select a candidate to review it.</div>
  </div>
</template>
