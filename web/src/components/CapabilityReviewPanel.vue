<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { message } from '../api/client'
import { registerCandidate, rejectCandidate, reviewCandidate } from '../api/candidates'
import type { CapabilityCandidate, CapabilityFields, RiskLevel } from '../api/types'

const props = defineProps<{ candidate: CapabilityCandidate }>()
const emit = defineEmits<{
  (event: 'changed', candidate: CapabilityCandidate): void
  (event: 'registered', candidateID: string): void
}>()

/** The form is seeded from the resolved view: extracted, then inferred, then review. */
function resolve(candidate: CapabilityCandidate): CapabilityFields {
  return { ...candidate.extracted, ...candidate.inferred, ...candidate.review }
}

const form = ref({
  name: '',
  description: '',
  business_domain: '',
  intents: '',
  owner: '',
  permissions: '',
  risk_level: 'low' as RiskLevel,
  reusable: true,
  note: '',
})

const busy = ref(false)
const error = ref('')
const notice = ref('')

watch(
  () => props.candidate,
  (candidate) => {
    const resolved = resolve(candidate)
    form.value = {
      name: resolved.name ?? candidate.name,
      description: resolved.description ?? candidate.description,
      business_domain: resolved.business_domain ?? '',
      intents: (resolved.intents ?? []).join(', '),
      owner: resolved.owner ?? '',
      permissions: (resolved.permissions ?? []).join(', '),
      risk_level: resolved.risk_level ?? 'low',
      reusable: resolved.reusable ?? true,
      note: '',
    }
    error.value = ''
    notice.value = ''
  },
  { immediate: true },
)

const isPending = computed(() =>
  ['extracted', 'inferred', 'needs_review'].includes(props.candidate.status),
)

/** Field names the system guessed at, so a reviewer knows what to check first. */
const inferredFields = computed(() => {
  const inferred = props.candidate.inferred ?? {}
  return Object.entries(inferred)
    .filter(([, value]) => value !== undefined && value !== '' && (!Array.isArray(value) || value.length > 0))
    .map(([key]) => key)
})

function splitList(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function fieldsFromForm(): CapabilityFields {
  return {
    name: form.value.name,
    description: form.value.description,
    business_domain: form.value.business_domain,
    intents: splitList(form.value.intents),
    owner: form.value.owner,
    permissions: splitList(form.value.permissions),
    risk_level: form.value.risk_level,
    reusable: form.value.reusable,
  }
}

async function save() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const detail = await reviewCandidate(props.candidate.id, fieldsFromForm(), form.value.note)
    notice.value = 'Review saved.'
    emit('changed', detail.candidate)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}

async function register() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    // Save first so the reviewer's edits are what gets registered.
    await reviewCandidate(props.candidate.id, fieldsFromForm(), form.value.note)
    const capability = await registerCandidate(props.candidate.id)
    notice.value = `Registered as ${capability.type}/${capability.slug}.`
    emit('registered', props.candidate.id)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}

async function reject() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const candidate = await rejectCandidate(props.candidate.id, form.value.note)
    notice.value = 'Candidate rejected.'
    emit('changed', candidate)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="card">
    <h3>Review</h3>

    <div class="row small" style="margin-bottom: 12px">
      <span class="tag plain">{{ candidate.candidate_type }}</span>
      <span class="tag">{{ candidate.status }}</span>
      <span class="muted mono">{{ candidate.external_id }}</span>
    </div>

    <div v-if="inferredFields.length" class="banner warn small" style="margin-bottom: 12px">
      The system inferred: <strong>{{ inferredFields.join(', ') }}</strong>. Everything else came
      straight from {{ candidate.source_system }}.
    </div>

    <div class="stack">
      <div>
        <label for="review-name">Name</label>
        <input id="review-name" v-model="form.name" type="text" :disabled="!isPending" />
      </div>
      <div>
        <label for="review-description">Description</label>
        <textarea id="review-description" v-model="form.description" :disabled="!isPending"></textarea>
      </div>
      <div class="row" style="gap: 12px">
        <div style="flex: 1 1 200px">
          <label for="review-domain">Business domain</label>
          <input id="review-domain" v-model="form.business_domain" type="text" :disabled="!isPending" />
        </div>
        <div style="flex: 1 1 200px">
          <label for="review-owner">Owner</label>
          <input id="review-owner" v-model="form.owner" type="text" :disabled="!isPending" />
        </div>
      </div>
      <div>
        <label for="review-intents">Intents (comma separated)</label>
        <input id="review-intents" v-model="form.intents" type="text" class="mono" :disabled="!isPending" />
      </div>
      <div>
        <label for="review-permissions">Permissions (comma separated)</label>
        <input
          id="review-permissions"
          v-model="form.permissions"
          type="text"
          class="mono"
          :disabled="!isPending"
        />
      </div>
      <div class="row" style="gap: 12px">
        <div style="flex: 1 1 160px">
          <label for="review-risk">Risk level</label>
          <select id="review-risk" v-model="form.risk_level" :disabled="!isPending">
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
          </select>
        </div>
        <div style="flex: 1 1 160px">
          <label for="review-reusable">Reusable</label>
          <select id="review-reusable" v-model="form.reusable" :disabled="!isPending">
            <option :value="true">yes</option>
            <option :value="false">no</option>
          </select>
        </div>
      </div>
      <div>
        <label for="review-note">Review note</label>
        <input id="review-note" v-model="form.note" type="text" :disabled="!isPending" />
      </div>

      <div class="row">
        <button class="primary" type="button" :disabled="busy || !isPending" @click="register">
          Register
        </button>
        <button type="button" :disabled="busy || !isPending" @click="save">Save review</button>
        <button class="danger" type="button" :disabled="busy || !isPending" @click="reject">
          Reject
        </button>
      </div>

      <div v-if="!isPending" class="banner small">
        This candidate is {{ candidate.status }} and can no longer be edited.
      </div>
      <div v-if="error" class="banner error small">{{ error }}</div>
      <div v-if="notice" class="banner ok small">{{ notice }}</div>
    </div>
  </div>
</template>
