<script setup lang="ts">
import { ref } from 'vue'
import { createSource } from '../api/sources'
import { message } from '../api/client'
import type { ExternalSource } from '../api/types'

const emit = defineEmits<{ (event: 'created', source: ExternalSource): void }>()

const name = ref('')
const type = ref('dify')
const baseURL = ref('')
const apiKey = ref('')
const busy = ref(false)
const error = ref('')
const created = ref<ExternalSource | null>(null)

async function submit() {
  busy.value = true
  error.value = ''
  created.value = null
  try {
    const config: Record<string, unknown> = {}
    if (apiKey.value) config.api_key = apiKey.value
    const source = await createSource({
      name: name.value,
      type: type.value,
      base_url: baseURL.value,
      auth_type: apiKey.value ? 'bearer' : '',
      config,
    })
    created.value = source
    emit('created', source)
    apiKey.value = ''
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="stack" @submit.prevent="submit">
    <div>
      <label for="source-name">Name</label>
      <input id="source-name" v-model="name" type="text" placeholder="customer-dify" required />
    </div>
    <div>
      <label for="source-type">Type</label>
      <select id="source-type" v-model="type">
        <option value="dify">Dify</option>
        <option value="mcp">MCP / Tool gateway</option>
        <option value="data">Data / Knowledge catalog</option>
        <option value="manual">Manual</option>
      </select>
    </div>
    <div>
      <label for="source-url">Base URL</label>
      <input id="source-url" v-model="baseURL" type="url" placeholder="https://dify.internal" />
    </div>
    <div>
      <label for="source-key">API key</label>
      <input id="source-key" v-model="apiKey" type="text" class="mono" placeholder="optional" />
      <!-- Credentials are stored on the source record and never read back out. -->
      <div class="small muted" style="margin-top: 4px">
        Stored on the source and redacted on every read.
      </div>
    </div>

    <div class="row">
      <button class="primary" type="submit" :disabled="busy || !name">
        {{ busy ? 'Saving…' : 'Save source' }}
      </button>
    </div>

    <div v-if="error" class="banner error small">{{ error }}</div>
    <div v-if="created" class="banner ok small">
      Saved <strong>{{ created.name }}</strong> as <span class="mono">{{ created.id }}</span>
    </div>
  </form>
</template>
