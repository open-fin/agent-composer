// Every backend response is wrapped in the same envelope. The client unwraps it once,
// here, so no component ever has to think about it.
export interface ApiError {
  code: string
  message: string
  details?: unknown
}

export interface Envelope<T> {
  success: boolean
  data: T | null
  error: ApiError | null
}

export interface Page<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

/** RequestFailed carries the server's error code so callers can branch on it. */
export class RequestFailed extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number,
    readonly details?: unknown,
  ) {
    super(message)
    this.name = 'RequestFailed'
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  let envelope: Envelope<T>
  try {
    envelope = (await response.json()) as Envelope<T>
  } catch {
    throw new RequestFailed('INTERNAL', `${response.status} ${response.statusText}`, response.status)
  }
  if (!envelope.success || envelope.error) {
    const error = envelope.error ?? { code: 'INTERNAL', message: 'request failed' }
    throw new RequestFailed(error.code, error.message, response.status, error.details)
  }
  return envelope.data as T
}

export async function get<T>(path: string): Promise<T> {
  return unwrap<T>(await fetch(path, { headers: { Accept: 'application/json' } }))
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
  return unwrap<T>(
    await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  )
}

export async function put<T>(path: string, body: unknown): Promise<T> {
  return unwrap<T>(
    await fetch(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(body),
    }),
  )
}

/** postFile uploads a document through the multipart path the file picker uses. */
export async function postFile<T>(path: string, file: File, sourceName: string): Promise<T> {
  const form = new FormData()
  form.append('file', file)
  form.append('source_name', sourceName)
  return unwrap<T>(await fetch(path, { method: 'POST', body: form }))
}

/** message renders any thrown value as something safe to show a user. */
export function message(error: unknown): string {
  if (error instanceof RequestFailed) return `${error.code}: ${error.message}`
  if (error instanceof Error) return error.message
  return String(error)
}
