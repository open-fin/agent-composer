import { get } from './client'
import type { Page } from './client'
import { toQueryString } from './candidates'
import type { Capability, CapabilityType, Dependency } from './types'

export interface CapabilityQuery {
  type?: CapabilityType | ''
  status?: string
  business_domain?: string
  q?: string
  limit?: number
  offset?: number
}

export function listCapabilities(query: CapabilityQuery = {}): Promise<Page<Capability>> {
  return get<Page<Capability>>(`/api/v1/capabilities${toQueryString(query)}`)
}

export function getCapability(id: string): Promise<Capability> {
  return get<Capability>(`/api/v1/capabilities/${id}`)
}

export function getDependencies(id: string): Promise<Dependency[]> {
  return get<Dependency[]>(`/api/v1/capabilities/${id}/dependencies`)
}
