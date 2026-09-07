<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get, message } from './api/client'

interface Readiness {
  status: string
  store: string
  enricher: string
}

const readiness = ref<Readiness | null>(null)
const readinessError = ref('')

onMounted(async () => {
  try {
    readiness.value = await get<Readiness>('/readyz')
  } catch (error) {
    readinessError.value = message(error)
  }
})
</script>

<template>
  <header class="app-header">
    <h1>Agent Composer</h1>
    <nav>
      <RouterLink to="/import">Source Import</RouterLink>
      <RouterLink to="/candidates">Extracted Capabilities</RouterLink>
      <RouterLink to="/catalog">Capability Catalog</RouterLink>
      <RouterLink to="/compose">Create Business Agent</RouterLink>
      <RouterLink to="/drafts">Business Agent Drafts</RouterLink>
    </nav>
    <span class="spacer"></span>
    <!-- Which enrichment path is live matters when reading the results, so it is shown. -->
    <span v-if="readiness" class="small muted">
      store: {{ readiness.store }} · enricher: {{ readiness.enricher }}
    </span>
    <span v-else-if="readinessError" class="small" style="color: var(--danger)">
      API unreachable
    </span>
  </header>

  <main class="app-main">
    <RouterView />
  </main>
</template>
