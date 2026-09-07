<script setup lang="ts">
import { computed } from 'vue'
import type { CapabilityRef, CompositionPlan } from '../api/types'

const props = defineProps<{ plan: CompositionPlan; advanced?: boolean }>()
const emit = defineEmits<{ (event: 'remove', ref: CapabilityRef): void }>()

interface Section {
  key: string
  label: string
  refs: CapabilityRef[]
}

const sections = computed<Section[]>(() =>
  [
    { key: 'agents', label: 'Agents', refs: props.plan.recommended_agents },
    { key: 'workflows', label: 'Workflows', refs: props.plan.recommended_workflows },
    { key: 'skills', label: 'Skills', refs: props.plan.recommended_skills },
    { key: 'tools', label: 'Tools', refs: props.plan.recommended_tools },
    { key: 'knowledge_data', label: 'Knowledge / Data', refs: props.plan.recommended_knowledge_data },
    { key: 'prompts', label: 'Prompts', refs: props.plan.recommended_prompts },
    { key: 'policies', label: 'Policies', refs: props.plan.recommended_policies },
  ].filter((section) => section.refs && section.refs.length > 0),
)

const selectedCount = computed(() =>
  sections.value.reduce((total, section) => total + section.refs.length, 0),
)

function severityClass(severity: string): string {
  if (severity === 'error') return 'banner error small'
  if (severity === 'warning') return 'banner warn small'
  return 'banner small'
}
</script>

<template>
  <div class="stack">
    <div class="card">
      <h3>Recommended composition ({{ selectedCount }} capabilities)</h3>

      <table>
        <thead>
          <tr>
            <th>Section</th>
            <th>Capability</th>
            <th>Why</th>
            <th v-if="advanced"></th>
          </tr>
        </thead>
        <tbody>
          <template v-for="section in sections" :key="section.key">
            <tr v-for="ref in section.refs" :key="ref.id">
              <td class="small muted">{{ section.label }}</td>
              <td>
                <span class="mono">{{ ref.slug }}</span>
                <div class="small muted">{{ ref.name }}</div>
              </td>
              <!-- Every capability says why it is here: a matched step, or a binding
                   that a matched capability requires. -->
              <td class="small muted">{{ ref.reason }}</td>
              <td v-if="advanced">
                <button class="chip danger" type="button" @click="emit('remove', ref)">Remove</button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div class="card">
      <h3>Workflow</h3>
      <div v-if="plan.workflow_steps.length" class="row small">
        <template v-for="(step, index) in plan.workflow_steps" :key="step">
          <span class="tag">{{ step }}</span>
          <span v-if="index < plan.workflow_steps.length - 1" class="muted">→</span>
        </template>
      </div>
      <div v-else class="muted small">No step could be satisfied.</div>
    </div>

    <div class="card">
      <h3>Governance</h3>
      <div class="row small" style="gap: 16px">
        <span>Risk: <span class="tag">{{ plan.governance.risk_level }}</span></span>
        <span>
          Permissions:
          <span v-for="permission in plan.governance.permissions" :key="permission" class="tag plain">
            {{ permission }}
          </span>
          <span v-if="!plan.governance.permissions.length" class="muted">none</span>
        </span>
        <span>
          Approvals:
          <span v-for="approval in plan.governance.approval_required" :key="approval" class="tag warn">
            {{ approval }}
          </span>
          <span v-if="!plan.governance.approval_required.length" class="muted">none</span>
        </span>
      </div>
      <div class="small muted" style="margin-top: 8px">
        Derived from the selected capabilities, not entered by hand.
      </div>
    </div>

    <div v-if="plan.missing_capabilities.length" class="card">
      <h3>Missing capabilities</h3>
      <p class="small muted" style="margin-top: -6px">
        Steps the goal needs that nothing in the registry covers. These are what to build next.
      </p>
      <table>
        <thead>
          <tr><th>Step</th><th>Needs</th><th>Suggestion</th></tr>
        </thead>
        <tbody>
          <tr v-for="gap in plan.missing_capabilities" :key="gap.step">
            <td class="mono">{{ gap.step }}</td>
            <td><span class="tag warn">{{ gap.required_type }}</span></td>
            <td class="small muted">{{ gap.suggestion }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="plan.warnings.length" class="card">
      <h3>Warnings</h3>
      <div class="stack">
        <div v-for="(warning, index) in plan.warnings" :key="index" :class="severityClass(warning.severity)">
          <strong class="mono">{{ warning.code }}</strong> — {{ warning.message }}
        </div>
      </div>
    </div>
  </div>
</template>
