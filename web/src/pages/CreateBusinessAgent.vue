<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from '../api/client'
import { recommendComposition, validateComposition } from '../api/compose'
import { createDraft } from '../api/drafts'
import CompositionRecommendation from '../components/CompositionRecommendation.vue'
import type { CapabilityRef, CompositionPlan, HarnessType, ValidationResult } from '../api/types'

const HARNESSES: Array<{ value: HarnessType; label: string; note: string }> = [
  { value: 'jiuwen_swarm', label: 'Jiuwen Swarm', note: 'Target only — no runtime in this release' },
  { value: 'dify_workflow', label: 'Dify Workflow', note: 'Sequential graph' },
  { value: 'http_agent', label: 'HTTP Agent', note: 'Single agent endpoint' },
  { value: 'manual', label: 'Manual', note: 'Hand-built by a team' },
]

const EXAMPLE_GOAL =
  'Create an RM campaign agent that analyzes customer profile, recommends suitable products, ' +
  'drafts a campaign message, and checks compliance before outreach.'

const router = useRouter()

const name = ref('RM Campaign Agent')
const goal = ref('')
const description = ref('')
const harness = ref<HarnessType>('jiuwen_swarm')
const advanced = ref(false)

const plan = ref<CompositionPlan | null>(null)
const validation = ref<ValidationResult | null>(null)
const busy = ref(false)
const error = ref('')

async function recommend() {
  busy.value = true
  error.value = ''
  validation.value = null
  try {
    plan.value = await recommendComposition({
      name: name.value,
      goal: goal.value,
      description: description.value,
      harness_type: harness.value,
    })
    validation.value = await validateComposition(plan.value)
  } catch (caught) {
    error.value = message(caught)
    plan.value = null
  } finally {
    busy.value = false
  }
}

/** Advanced mode lets a reviewer drop a capability; the plan is then re-validated. */
async function removeRef(target: CapabilityRef) {
  if (!plan.value) return
  const sections: Array<keyof CompositionPlan> = [
    'recommended_agents',
    'recommended_workflows',
    'recommended_skills',
    'recommended_tools',
    'recommended_knowledge_data',
    'recommended_prompts',
    'recommended_policies',
  ]
  for (const section of sections) {
    const refs = plan.value[section] as CapabilityRef[]
    plan.value[section] = refs.filter((ref) => ref.id !== target.id) as never
  }
  try {
    validation.value = await validateComposition(plan.value)
  } catch (caught) {
    error.value = message(caught)
  }
}

async function generate() {
  if (!plan.value) return
  busy.value = true
  error.value = ''
  try {
    const draft = await createDraft({
      name: name.value,
      goal: goal.value,
      description: description.value,
      harness_type: harness.value,
      plan: plan.value,
    })
    router.push(`/drafts/${draft.id}`)
  } catch (caught) {
    error.value = message(caught)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <h2 class="page-title">Create Business Agent</h2>
  <p class="page-subtitle">
    Describe the outcome you want. The goal is decomposed into the steps it implies, and each
    step is matched against the registry — so the answer includes both what exists and what
    does not.
  </p>

  <div class="card" style="margin-bottom: 16px">
    <div class="stack">
      <div class="row" style="gap: 12px; align-items: flex-end">
        <div style="flex: 1 1 260px">
          <label for="agent-name">Agent name</label>
          <input id="agent-name" v-model="name" type="text" />
        </div>
        <div style="flex: 1 1 260px">
          <label for="harness">Target harness</label>
          <select id="harness" v-model="harness">
            <option v-for="entry in HARNESSES" :key="entry.value" :value="entry.value">
              {{ entry.label }} — {{ entry.note }}
            </option>
          </select>
        </div>
      </div>

      <div>
        <label for="goal">Business goal</label>
        <textarea
          id="goal"
          v-model="goal"
          rows="3"
          placeholder="What should this agent achieve, end to end?"
        ></textarea>
        <button class="chip" type="button" style="margin-top: 6px" @click="goal = EXAMPLE_GOAL">
          Use the demo goal
        </button>
      </div>

      <div>
        <label for="agent-description">Description (optional)</label>
        <input id="agent-description" v-model="description" type="text" />
      </div>

      <div class="row">
        <button class="primary" type="button" :disabled="busy || !goal.trim()" @click="recommend">
          {{ busy ? 'Working…' : 'Recommend Composition' }}
        </button>
        <button
          v-if="plan"
          class="primary"
          type="button"
          :disabled="busy || !(validation?.valid ?? false)"
          @click="generate"
        >
          Generate Draft
        </button>
        <label class="row small muted" style="margin: 0; gap: 6px; font-weight: 500">
          <input v-model="advanced" type="checkbox" style="width: auto" />
          Advanced: edit the composition by hand
        </label>
      </div>
    </div>
  </div>

  <div v-if="error" class="banner error" style="margin-bottom: 16px">{{ error }}</div>

  <div v-if="validation && !validation.valid" class="banner error" style="margin-bottom: 16px">
    <strong>This composition cannot become a draft yet.</strong>
    <div v-for="(issue, index) in validation.errors" :key="index" class="small">
      {{ issue.message }}
    </div>
  </div>

  <CompositionRecommendation v-if="plan" :plan="plan" :advanced="advanced" @remove="removeRef" />
  <div v-else class="card empty">
    Enter a goal and choose Recommend Composition.
  </div>
</template>
