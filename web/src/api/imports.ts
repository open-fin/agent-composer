import { post, postFile } from './client'
import type { ImportResult } from './types'

/** The four import paths, in the order the Source Import page presents them. */
export const IMPORT_KINDS = [
  {
    id: 'dify',
    label: 'Dify DSL',
    path: '/api/v1/import/dify/dsl',
    accepts: '.yaml,.yml',
    sourceName: 'customer-dify',
    blurb: 'Upload a Dify application export. One app yields the agent, its workflow, its tools, its knowledge bases, its prompts and the skills behind them.',
  },
  {
    id: 'manual',
    label: 'Manual YAML',
    path: '/api/v1/import/manual',
    accepts: '.yaml,.yml',
    sourceName: 'manual-registration',
    blurb: 'Register a capability that lives in a system with no importer, by describing it directly.',
  },
  {
    id: 'mcp',
    label: 'Mock MCP / Tool Gateway',
    path: '/api/v1/import/mcp/mock',
    accepts: '',
    sourceName: 'enterprise-tool-gateway',
    blurb: 'Pull a tool inventory from an MCP server. This build ships a fixed inventory so the path can be shown without a gateway.',
  },
  {
    id: 'data',
    label: 'Mock Knowledge / Data',
    path: '/api/v1/import/data/mock',
    accepts: '',
    sourceName: 'enterprise-data-catalog',
    blurb: 'Pull datasets from an enterprise data catalog. This build ships a fixed catalog.',
  },
] as const

export type ImportKindId = (typeof IMPORT_KINDS)[number]['id']

export function importText(path: string, sourceName: string, content: string): Promise<ImportResult> {
  return post<ImportResult>(path, { source_name: sourceName, content })
}

export function importFile(path: string, sourceName: string, file: File): Promise<ImportResult> {
  return postFile<ImportResult>(path, file, sourceName)
}

export function importMock(path: string, sourceName: string): Promise<ImportResult> {
  return post<ImportResult>(path, { source_name: sourceName })
}
