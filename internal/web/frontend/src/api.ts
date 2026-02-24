export type Container = {
  id: string
  name: string
  state: string
  template: string
  project_path: string
  remote_user: string
  created_at: string
  sessions: Array<Session>
}

export type Session = {
  name: string
  windows: number
  attached: boolean
}

export type WorktreeResponse = {
  name: string
  path: string
  is_main: boolean
  container: Container | null
}

export type ProjectResponse = {
  name: string
  path: string
  encoded_path: string
  has_makefile: boolean
  worktrees: Array<WorktreeResponse>
}

export type ProjectsListResponse = {
  projects: Array<ProjectResponse>
  unmatched: Array<Container>
}

const API_BASE = '/api'

export async function fetchContainers(): Promise<Array<Container>> {
  const res = await fetch(`${API_BASE}/containers`)
  if (!res.ok) throw new Error(`failed to fetch containers: ${res.status}`)
  return res.json() as Promise<Array<Container>>
}

export async function fetchContainer(id: string): Promise<Container> {
  const res = await fetch(`${API_BASE}/containers/${id}`)
  if (!res.ok) throw new Error(`failed to fetch container: ${res.status}`)
  return res.json() as Promise<Container>
}

export async function fetchSessions(containerId: string): Promise<Array<Session>> {
  const res = await fetch(`${API_BASE}/containers/${containerId}/sessions`)
  if (!res.ok) throw new Error(`failed to fetch sessions: ${res.status}`)
  return res.json() as Promise<Array<Session>>
}

export async function fetchProjects(): Promise<ProjectsListResponse> {
  const res = await fetch(`${API_BASE}/projects`)
  if (!res.ok) throw new Error(`failed to fetch projects: ${res.status}`)
  return res.json() as Promise<ProjectsListResponse>
}

export async function startContainer(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/containers/${id}/start`, {
    method: 'POST',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to start container: ${res.status}`)
  }
}

export async function stopContainer(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/containers/${id}/stop`, {
    method: 'POST',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to stop container: ${res.status}`)
  }
}

export async function destroyContainer(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/containers/${id}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to destroy container: ${res.status}`)
  }
}

export async function createSession(containerId: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/containers/${containerId}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to create session: ${res.status}`)
  }
}

export async function destroySession(containerId: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/containers/${containerId}/sessions/${name}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to destroy session: ${res.status}`)
  }
}

export async function fetchHostSessions(): Promise<Array<Session>> {
  const res = await fetch(`${API_BASE}/host/sessions`)
  if (!res.ok) throw new Error(`failed to fetch host sessions: ${res.status}`)
  return res.json() as Promise<Array<Session>>
}

export async function createHostSession(name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/host/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to create host session: ${res.status}`)
  }
}

export async function destroyHostSession(name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/host/sessions/${name}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to destroy host session: ${res.status}`)
  }
}

export async function createWorktree(encodedPath: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/projects/${encodedPath}/worktrees`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to create worktree: ${res.status}`)
  }
}

export async function deleteWorktree(encodedPath: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/projects/${encodedPath}/worktrees/${name}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to delete worktree: ${res.status}`)
  }
}

export async function startWorktreeContainer(encodedPath: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/projects/${encodedPath}/worktrees/${name}/start`, {
    method: 'POST',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error ?? `failed to start worktree container: ${res.status}`)
  }
}
