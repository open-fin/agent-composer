// Mirrors of the Go domain types the UI consumes.

export type CapabilityType =
  | 'agent'
  | 'workflow'
  | 'skill'
  | 'tool'
  | 'knowledge_data'
  | 'prompt_template'
  | 'policy'

export type RiskLevel = 'low' | 'medium' | 'high'

export type CandidateStatus = 'extracted' | 'inferred' | 'needs_review' | 'registered' | 'rejected'

export type HarnessType = 'dify_workflow' | 'jiuwen_swarm' | 'http_agent' | 'manual'

export type DraftStatus = 'draft' | 'needs_review' | 'ready_for_eval'

export interface CapabilityFields {
  name?: string
  description?: string
  subtype?: string
  business_domain?: string
  intents?: string[]
  tags?: string[]
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  owner?: string
  permissions?: string[]
  risk_level?: RiskLevel
  reusable?: boolean
  metadata?: Record<string, unknown>
}

export interface CapabilityCandidate {
  id: string
  source_id: string
  source_system: string
  external_id: string
  candidate_type: CapabilityType
  name: string
  description: string
  raw_payload: Record<string, unknown>
  extracted: CapabilityFields
  inferred: CapabilityFields
  review: CapabilityFields
  status: CandidateStatus
  created_at: string
  updated_at: string
}

export interface Dependency {
  id: string
  capability_id: string
  depends_on_capability_id: string
  dependency_type: string
  depends_on_slug?: string
  depends_on_type?: CapabilityType
  depends_on_name?: string
}

export interface Capability {
  id: string
  slug: string
  name: string
  type: CapabilityType
  subtype?: string
  description: string
  source_system: string
  external_id?: string
  business_domain?: string
  intents: string[]
  tags: string[]
  input_schema: Record<string, unknown>
  output_schema: Record<string, unknown>
  owner?: string
  permissions: string[]
  risk_level: RiskLevel
  reusable: boolean
  status: string
  metadata: Record<string, unknown>
  dependencies?: Dependency[]
}

export interface ExternalSource {
  id: string
  name: string
  type: string
  base_url?: string
  auth_type?: string
  config: Record<string, unknown>
  status: string
}

export interface ImportResult {
  source: ExternalSource
  candidates: CapabilityCandidate[]
  warnings: string[]
}

export interface CapabilityRef {
  id: string
  slug: string
  name: string
  type: CapabilityType
  role?: string
  score?: number
  reason?: string
}

export interface RequiredRole {
  step: string
  capability_type: CapabilityType
  description?: string
  keywords?: string[]
  intents?: string[]
}

export interface MissingCapability {
  step: string
  required_type: CapabilityType
  reason: string
  suggestion?: string
  keywords?: string[]
}

export interface Warning {
  code: string
  severity: 'info' | 'warning' | 'error'
  message: string
  capability_slug?: string
  step?: string
}

export interface Governance {
  risk_level: RiskLevel
  approval_required: string[]
  permissions: string[]
}

export interface CompositionPlan {
  goal: { name: string; goal: string; description?: string; harness_type: HarnessType }
  roles: RequiredRole[]
  recommended_agents: CapabilityRef[]
  recommended_workflows: CapabilityRef[]
  recommended_skills: CapabilityRef[]
  recommended_tools: CapabilityRef[]
  recommended_knowledge_data: CapabilityRef[]
  recommended_prompts: CapabilityRef[]
  recommended_policies: CapabilityRef[]
  workflow_steps: string[]
  missing_capabilities: MissingCapability[]
  warnings: Warning[]
  governance: Governance
}

export interface ValidationResult {
  valid: boolean
  errors: Warning[]
  warnings: Warning[]
}

export interface CompositionComponent {
  id: string
  capability_id: string
  component_role: string
  order_index: number
  capability_slug?: string
  capability_name?: string
  capability_type?: CapabilityType
}

export interface BusinessAgentDraft {
  id: string
  slug: string
  name: string
  goal: string
  description?: string
  harness_type: HarnessType
  status: DraftStatus
  composition: CompositionPlan
  governance: Governance
  yaml_text: string
  components?: CompositionComponent[]
  created_at: string
  updated_at: string
}
