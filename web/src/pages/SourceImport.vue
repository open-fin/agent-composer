<script setup lang="ts">
import { computed, ref } from 'vue'
import { message } from '../api/client'
import { IMPORT_KINDS, importFile, importMock, importText } from '../api/imports'
import SourceConnectorForm from '../components/SourceConnectorForm.vue'
import type { ImportResult } from '../api/types'

const selectedKind = ref<string>(IMPORT_KINDS[0].id)
const kind = computed(() => IMPORT_KINDS.find((entry) => entry.id === selectedKind.value)!)
const needsDocument = computed(() => kind.value.accepts !== '')

const pastedContent = ref('')
const chosenFile = ref<File | null>(null)
const busy = ref(false)
const error = ref('')
const result = ref<ImportResult | null>(null)
const showConnector = ref(false)

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  chosenFile.value = input.files?.[0] ?? null
}

const canImport = computed(() => {
  if (busy.value) return false
  if (!needsDocument.value) return true
  return Boolean(chosenFile.value) || pastedContent.value.trim().length > 0
})

async function runImport() {
  busy.value = true
  error.value = ''
  result.value = null
  try {
    const { path, sourceName } = kind.value
    if (!needsDocument.value) {
      result.value = await importMock(path, sourceName)
    } else if (chosenFile.value) {
      result.value = await importFile(path, sourceName, chosenFile.value)
    } else {
      result.value = await importText(path, sourceName, pastedContent.value)
    }
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}

function statusClass(status: string): string {
  if (status === 'registered') return 'tag ok'
  if (status === 'needs_review') return 'tag warn'
  if (status === 'rejected') return 'tag plain'
  return 'tag'
}
</script>

<template>
  <h2 class="page-title">Source Import</h2>
  <p class="page-subtitle">
    Bring in capabilities the business already owns. Nothing is registered here: an import
    produces candidates, which are reviewed on the next page before they enter the registry.
  </p>

  <div class="split">
    <div class="stack">
      <div class="card">
        <h3>Import method</h3>
        <div class="row" style="margin-bottom: 12px">
          <button
            v-for="entry in IMPORT_KINDS"
            :key="entry.id"
            type="button"
            class="chip"
            :class="{ active: entry.id === selectedKind }"
            @click="selectedKind = entry.id"
          >
            {{ entry.label }}
          </button>
        </div>
        <p class="small muted">{{ kind.blurb }}</p>

        <template v-if="needsDocument">
          <div style="margin-top: 12px">
            <label :for="'file-' + kind.id">Upload a document</label>
            <input :id="'file-' + kind.id" type="file" :accept="kind.accepts" @change="onFileChange" />
          </div>
          <div style="margin-top: 12px">
            <label :for="'paste-' + kind.id">…or paste it</label>
            <textarea
              :id="'paste-' + kind.id"
              v-model="pastedContent"
              class="mono"
              rows="8"
              placeholder="app:&#10;  name: Customer Insight Agent&#10;  mode: advanced-chat"
            ></textarea>
          </div>
        </template>

        <div class="row" style="margin-top: 14px">
          <button class="primary" type="button" :disabled="!canImport" @click="runImport">
            {{ busy ? 'Importing…' : 'Import' }}
          </button>
          <span class="small muted">source: {{ kind.sourceName }}</span>
        </div>

        <div v-if="error" class="banner error small" style="margin-top: 12px">{{ error }}</div>
      </div>

      <div class="card">
        <div class="row" style="justify-content: space-between">
          <h3 style="margin: 0">Configure a source connection</h3>
          <button class="chip" type="button" @click="showConnector = !showConnector">
            {{ showConnector ? 'Hide' : 'Show' }}
          </button>
        </div>
        <p class="small muted" style="margin-top: 8px">
          Optional. Uploads create their source automatically; configure one explicitly when you
          want to record a base URL and credentials for a system.
        </p>
        <SourceConnectorForm v-if="showConnector" />
      </div>
    </div>

    <div class="card">
      <h3>Capability candidates</h3>
      <div v-if="!result" class="empty">Import something to see the candidates it produces.</div>
      <template v-else>
        <div class="banner ok small" style="margin-bottom: 12px">
          {{ result.candidates.length }} candidate(s) from
          <strong>{{ result.source.name }}</strong>
        </div>
        <div v-if="result.warnings.length" class="banner warn small" style="margin-bottom: 12px">
          <div v-for="warning in result.warnings" :key="warning">{{ warning }}</div>
        </div>
        <table>
          <thead>
            <tr><th>Type</th><th>Identifier</th><th>Name</th><th>Status</th></tr>
          </thead>
          <tbody>
            <tr v-for="candidate in result.candidates" :key="candidate.id">
              <td><span class="tag plain">{{ candidate.candidate_type }}</span></td>
              <td class="mono small">{{ candidate.external_id }}</td>
              <td>{{ candidate.name }}</td>
              <td><span :class="statusClass(candidate.status)">{{ candidate.status }}</span></td>
            </tr>
          </tbody>
        </table>
        <div class="row" style="margin-top: 14px">
          <RouterLink to="/candidates"><button class="primary" type="button">Review candidates</button></RouterLink>
        </div>
      </template>
    </div>
  </div>
</template>
