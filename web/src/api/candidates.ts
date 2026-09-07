import { get, post } from './client'
import type { Page } from './client'
import type { CapabilityCandidate, CapabilityFields, Capability } from './types'

export interface CandidateQuery {
  status?: string
  type?: string
  source_id?: string
  q?: string
  limit?: number
  offset?: number
}

function toQueryString(query: object): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ''
}

export function listCandidates(query: CandidateQuery = {}): Promise<Page<CapabilityCandidate>> {
  return get<Page<CapabilityCandidate>>(`/api/v1/candidates${toQueryString(query)}`)
}

export interface CandidateDetail {
  candidate: CapabilityCandidate
  resolved: CapabilityFields
}

export function getCandidate(id: string): Promise<CandidateDetail> {
  return get<CandidateDetail>(`/api/v1/candidates/${id}`)
}

export function reviewCandidate(id: string, fields: CapabilityFields, note = ''): Promise<CandidateDetail> {
  return post<CandidateDetail>(`/api/v1/candidates/${id}/review`, { fields, note })
}

export function registerCandidate(id: string): Promise<Capability> {
  return post<Capability>(`/api/v1/candidates/${id}/register`)
}

export function rejectCandidate(id: string, reason = ''): Promise<CapabilityCandidate> {
  return post<CapabilityCandidate>(`/api/v1/candidates/${id}/reject`, { reason })
}

export { toQueryString }
