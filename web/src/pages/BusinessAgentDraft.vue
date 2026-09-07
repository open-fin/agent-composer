<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from '../api/client'
import { createDraft, downloadURL, getDraft, listDrafts, updateDraft } from '../api/drafts'
import DraftYamlViewer from '../components/DraftYamlViewer.vue'
import type { BusinessAgentDraft, DraftStatus } from '../api/types'

const DRAFT_STATUSES: DraftStatus[] = ['draft', 'needs_review', 'ready_for_eval']

const route = useRoute()
const router = useRouter()

const drafts = ref<BusinessAgentDraft[]>([])
const draft = ref<BusinessAgentDraft | null>(null)
const editing = ref(false)
const editName = ref('')
const editStatus = ref<DraftStatus>('draft')
const busy = ref(false)
const error = ref('')
const notice = ref('')

async function loadList() {
  try {
    const page = await listDrafts({})
    drafts.value = page.items
  } catch (caught) {
    error.value = message(caught)
  }
}

async function loadDraft(id: string) {
  error.value = ''
  try {
    draft.value = await getDraft(id)
    editName.value = draft.value.name
    editStatus.value = draft.value.status
  } catch (caught) {
    error.value = message(caught)
    draft.value = null
  }
}

async function sync() {
  await loadList()
  const id = route.params.id as string | undefined
  if (id) {
    await loadDraft(id)
  } else if (drafts.value.length) {
    router.replace(`/drafts/${drafts.value[drafts.value.length - 1].id}`)
  } else {
    draft.value = null
  }
}

onMounted(sync)
watch(() => route.params.id, sync)

const componentsByRole = computed(() => {
  const grouped = new Map<string, BusinessAgentDraft['components']>()
  for (const component of draft.value?.components ?? []) {
    const existing = grouped.get(component.component_role) ?? []
    existing.push(component)
    grouped.set(component.component_role, existing)
  }
  return Array.from(grouped.entries())
})

async function save() {
  if (!draft.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    draft.value = await updateDraft(draft.value.id, { name: editName.value, status: editStatus.value })
    editing.value = false
    notice.value = 'Draft updated.'
    await loadList()
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}

/** Clone re-runs generation from the same plan, giving an independent draft to edit. */
async function clone() {
  if (!draft.value) return
  busy.value = true
  error.value = ''
  try {
    const copy = await createDraft({
      name: `${draft.value.name} (copy)`,
      goal: draft.value.goal,
      description: draft.value.description,
      harness_type: draft.value.harness_type,
      plan: draft.value.composition,
    })
    await loadList()
    router.push(`/drafts/${copy.id}`)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}

function statusClass(status: string): string {
  if (status === 'ready_for_eval') return 'tag ok'
  if (status === 'needs_review') return 'tag warn'
  return 'tag plain'
}
</script>

<template>
  <h2 class="page-title">Business Agent Draft</h2>
  <p class="page-subtitle">
    The phase 1 deliverable. A draft is never published or executed; ready_for_eval is the
    handoff point to evaluation.
  </p>

  <div v-if="error" class="banner error" style="margin-bottom: 16px">{{ error }}</div>
  <div v-if="notice" class="banner ok" style="margin-bottom: 16px">{{ notice }}</div>

  <div v-if="!drafts.length" class="card empty">
    No drafts yet. Compose one on the Create Business Agent page.
  </div>

  <template v-else>
    <div class="card" style="margin-bottom: 16px">
      <div class="row">
        <RouterLink v-for="entry in drafts" :key="entry.id" :to="`/drafts/${entry.id}`">
          <button class="chip" type="button" :class="{ active: entry.id === draft?.id }">
            {{ entry.name }}
          </button>
        </RouterLink>
      </div>
    </div>

    <div v-if="draft" class="split">
      <div class="stack">
        <div class="card">
          <div class="row" style="justify-content: space-between">
            <h3 style="margin: 0">{{ draft.name }}</h3>
            <span :class="statusClass(draft.status)">{{ draft.status }}</span>
          </div>

          <table style="margin-top: 12px">
            <tbody>
              <tr><th>Identifier</th><td class="mono">{{ draft.slug }}</td></tr>
              <tr><th>Goal</th><td>{{ draft.goal }}</td></tr>
              <tr><th>Description</th><td>{{ draft.description || '—' }}</td></tr>
              <tr><th>Harness</th><td>{{ draft.harness_type }}</td></tr>
            </tbody>
          </table>

          <div class="row" style="margin-top: 12px">
            <button type="button" @click="editing = !editing">{{ editing ? 'Cancel' : 'Edit' }}</button>
            <button type="button" :disabled="busy" @click="clone">Clone</button>
          </div>

          <div v-if="editing" class="stack" style="margin-top: 12px">
            <div>
              <label for="draft-name">Name</label>
              <input id="draft-name" v-model="editName" type="text" />
            </div>
            <div>
              <label for="draft-status">Status</label>
              <select id="draft-status" v-model="editStatus">
                <option v-for="status in DRAFT_STATUSES" :key="status" :value="status">{{ status }}</option>
              </select>
            </div>
            <div class="row">
              <button class="primary" type="button" :disabled="busy" @click="save">Save</button>
            </div>
          </div>
        </div>

        <div class="card">
          <h3>Selected capabilities</h3>
          <div v-for="[role, components] in componentsByRole" :key="role" style="margin-bottom: 10px">
            <div class="small muted">{{ role }}</div>
            <div class="row">
              <span v-for="component in components" :key="component!.id" class="tag">
                {{ component!.capability_slug }}
              </span>
            </div>
          </div>
          <div v-if="!componentsByRole.length" class="muted small">No components recorded.</div>
        </div>

        <div class="card">
          <h3>Workflow steps</h3>
          <div class="row small">
            <template v-for="(step, index) in draft.composition.workflow_steps" :key="step">
              <span class="tag">{{ step }}</span>
              <span v-if="index < draft.composition.workflow_steps.length - 1" class="muted">→</span>
            </template>
          </div>
        </div>

        <div class="card">
          <h3>Governance</h3>
          <table>
            <tbody>
              <tr><th>Risk level</th><td>{{ draft.governance.risk_level }}</td></tr>
              <tr>
                <th>Approvals</th>
                <td>
                  <span v-for="approval in draft.governance.approval_required" :key="approval" class="tag warn">
                    {{ approval }}
                  </span>
                  <span v-if="!draft.governance.approval_required.length" class="muted">none</span>
                </td>
              </tr>
              <tr>
                <th>Permissions</th>
                <td>
                  <span v-for="permission in draft.governance.permissions" :key="permission" class="tag plain">
                    {{ permission }}
                  </span>
                  <span v-if="!draft.governance.permissions.length" class="muted">none</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="draft.composition.missing_capabilities?.length" class="card">
          <h3>Still missing</h3>
          <div v-for="gap in draft.composition.missing_capabilities" :key="gap.step" class="banner warn small" style="margin-bottom: 8px">
            <strong class="mono">{{ gap.step }}</strong> needs a {{ gap.required_type }} capability.
          </div>
        </div>
      </div>

      <DraftYamlViewer
        :yaml="draft.yaml_text"
        :download-href="downloadURL(draft.id)"
        :filename="`${draft.slug}.yaml`"
      />
    </div>
  </template>
</template>
