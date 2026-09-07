import { get, post, put } from './client'
import type { Page } from './client'
import { toQueryString } from './candidates'
import type { BusinessAgentDraft, CompositionPlan, DraftStatus, HarnessType } from './types'

export interface CreateDraftInput {
  name: string
  goal: string
  description?: string
  harness_type: HarnessType
  plan: CompositionPlan
}

export function createDraft(input: CreateDraftInput): Promise<BusinessAgentDraft> {
  return post<BusinessAgentDraft>('/api/v1/drafts', input)
}

export function listDrafts(query: { status?: string; q?: string } = {}): Promise<Page<BusinessAgentDraft>> {
  return get<Page<BusinessAgentDraft>>(`/api/v1/drafts${toQueryString(query)}`)
}

export function getDraft(id: string): Promise<BusinessAgentDraft> {
  return get<BusinessAgentDraft>(`/api/v1/drafts/${id}`)
}

export interface DraftUpdate {
  name?: string
  description?: string
  goal?: string
  harness_type?: HarnessType
  status?: DraftStatus
}

export function updateDraft(id: string, update: DraftUpdate): Promise<BusinessAgentDraft> {
  return put<BusinessAgentDraft>(`/api/v1/drafts/${id}`, update)
}

export interface ExportedYAML {
  yaml: string
  filename: string
}

export function getDraftYAML(id: string): Promise<ExportedYAML> {
  return get<ExportedYAML>(`/api/v1/drafts/${id}/yaml`)
}

/** downloadURL points at the raw attachment form of the export endpoint. */
export function downloadURL(id: string): string {
  return `/api/v1/drafts/${id}/yaml?download=1`
}
