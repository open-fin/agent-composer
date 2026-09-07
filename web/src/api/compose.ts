import { post } from './client'
import type { CompositionPlan, HarnessType, ValidationResult } from './types'

export interface RecommendInput {
  name: string
  goal: string
  description?: string
  harness_type: HarnessType
}

export function recommendComposition(input: RecommendInput): Promise<CompositionPlan> {
  return post<CompositionPlan>('/api/v1/compositions/recommend', input)
}

export function validateComposition(plan: CompositionPlan): Promise<ValidationResult> {
  return post<ValidationResult>('/api/v1/compositions/validate', { plan })
}
