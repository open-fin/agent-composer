import { get, post } from './client'
import type { ExternalSource } from './types'

export function listSources(): Promise<ExternalSource[]> {
  return get<ExternalSource[]>('/api/v1/sources')
}

export function getSource(id: string): Promise<ExternalSource> {
  return get<ExternalSource>(`/api/v1/sources/${id}`)
}

export interface CreateSourceInput {
  name: string
  type: string
  base_url?: string
  auth_type?: string
  config?: Record<string, unknown>
}

export function createSource(input: CreateSourceInput): Promise<ExternalSource> {
  return post<ExternalSource>('/api/v1/sources', input)
}
