<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { message } from '../api/client'
import { getCapability, listCapabilities } from '../api/capabilities'
import CapabilityCard from '../components/CapabilityCard.vue'
import type { Capability, CapabilityType } from '../api/types'

const TYPES: Array<CapabilityType | ''> = [
  '',
  'agent',
  'workflow',
  'skill',
  'tool',
  'knowledge_data',
  'prompt_template',
  'policy',
]

const capabilities = ref<Capability[]>([])
const selected = ref<Capability | null>(null)
const typeFilter = ref<CapabilityType | ''>('')
const search = ref('')
const total = ref(0)
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const page = await listCapabilities({ type: typeFilter.value, q: search.value, limit: 200 })
    capabilities.value = page.items
    total.value = page.total
    await select(capabilities.value.find((item) => item.id === selected.value?.id) ?? capabilities.value[0] ?? null)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    loading.value = false
  }
}

/** Detail is fetched separately so the panel shows hydrated dependencies. */
async function select(capability: Capability | null) {
  if (!capability) {
    selected.value = null
    return
  }
  try {
    selected.value = await getCapability(capability.id)
  } catch (caught) {
    error.value = message(caught)
    selected.value = capability
  }
}

onMounted(load)
watch([typeFilter, search], load)

function pretty(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2)
}
</script>

<template>
  <h2 class="page-title">Capability Catalog</h2>
  <p class="page-subtitle">
    Everything registered and reusable. These are the parts a business agent is composed from.
  </p>

  <div class="card" style="margin-bottom: 16px">
    <div class="row">
      <button
        v-for="type in TYPES"
        :key="type || 'all'"
        type="button"
        class="chip"
        :class="{ active: typeFilter === type }"
        @click="typeFilter = type"
      >
        {{ type || 'all' }}
      </button>
      <span style="flex: 1"></span>
      <input v-model="search" type="text" placeholder="Search name, slug, intent or tag" style="max-width: 300px" />
    </div>
    <div class="small muted" style="margin-top: 8px">{{ total }} capabilities</div>
  </div>

  <div v-if="error" class="banner error" style="margin-bottom: 16px">{{ error }}</div>

  <div class="split">
    <div>
      <div v-if="loading && !capabilities.length" class="card empty">Loading…</div>
      <div v-else-if="!capabilities.length" class="card empty">
        Nothing registered yet. Review and register candidates first.
      </div>
      <div v-else class="list">
        <CapabilityCard
          v-for="capability in capabilities"
          :key="capability.id"
          :capability="capability"
          :selected="capability.id === selected?.id"
          @select="select($event)"
        />
      </div>
    </div>

    <div v-if="selected" class="card">
      <h3>{{ selected.name }}</h3>
      <table>
        <tbody>
          <tr><th>Slug</th><td class="mono">{{ selected.slug }}</td></tr>
          <tr><th>Type</th><td>{{ selected.type }}<span v-if="selected.subtype"> / {{ selected.subtype }}</span></td></tr>
          <tr><th>Source</th><td>{{ selected.source_system }} <span class="muted mono">{{ selected.external_id }}</span></td></tr>
          <tr><th>Domain</th><td>{{ selected.business_domain || '—' }}</td></tr>
          <tr><th>Owner</th><td>{{ selected.owner || '—' }}</td></tr>
          <tr><th>Risk</th><td>{{ selected.risk_level }}</td></tr>
          <tr><th>Reusable</th><td>{{ selected.reusable ? 'yes' : 'no' }}</td></tr>
          <tr>
            <th>Intents</th>
            <td>
              <span v-for="intent in selected.intents" :key="intent" class="tag">{{ intent }}</span>
              <span v-if="!selected.intents.length" class="muted">—</span>
            </td>
          </tr>
          <tr>
            <th>Permissions</th>
            <td>
              <span v-for="permission in selected.permissions" :key="permission" class="tag plain">{{ permission }}</span>
              <span v-if="!selected.permissions.length" class="muted">—</span>
            </td>
          </tr>
          <tr>
            <th>Dependencies</th>
            <td>
              <div v-for="dependency in selected.dependencies ?? []" :key="dependency.id" class="small">
                <span class="tag plain">{{ dependency.dependency_type }}</span>
                <span class="mono"> {{ dependency.depends_on_slug }}</span>
              </div>
              <span v-if="!(selected.dependencies ?? []).length" class="muted">—</span>
            </td>
          </tr>
        </tbody>
      </table>

      <h3 style="margin-top: 16px">Input schema</h3>
      <pre class="yaml">{{ pretty(selected.input_schema) }}</pre>
      <h3 style="margin-top: 16px">Output schema</h3>
      <pre class="yaml">{{ pretty(selected.output_schema) }}</pre>
    </div>
    <div v-else class="card empty">Select a capability to see its detail.</div>
  </div>
</template>
