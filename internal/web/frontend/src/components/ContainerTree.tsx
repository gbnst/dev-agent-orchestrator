import { useState, useEffect, useCallback } from 'react'
import { type ProjectsListResponse, fetchProjects } from '../api'
import { useServerEvents } from '../lib/useServerEvents'
import { ContainerCard } from './ContainerCard'
import { ProjectCard } from './ProjectCard'
import { HostCard, HOST_ID } from './HostCard'

const STORAGE_KEY = 'devagent-expanded-cards'

function loadExpanded(): Set<string> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return new Set(JSON.parse(raw) as string[])
  } catch { /* ignore corrupt data */ }
  return new Set()
}

function saveExpanded(ids: Set<string>) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
}

type ContainerTreeProps = {
  readonly onAttach: (containerId: string, containerName: string, sessionName: string) => void
}

export function ContainerTree({ onAttach }: ContainerTreeProps) {
  const [data, setData] = useState<ProjectsListResponse>({ projects: [], unmatched: [] })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(loadExpanded)

  const load = useCallback(async () => {
    try {
      const result = await fetchProjects()
      setData(result)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load projects')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  useServerEvents(load)

  // Prune stale IDs whenever the projects or unmatched containers change.
  useEffect(() => {
    const validIds = new Set<string>([
      HOST_ID,
      ...data.projects.map(p => p.encoded_path),
      ...data.unmatched.map(c => c.id),
    ])
    setExpandedIds(prev => {
      const pruned = new Set([...prev].filter(id => validIds.has(id)))
      if (pruned.size !== prev.size) {
        saveExpanded(pruned)
      }
      return pruned.size !== prev.size ? pruned : prev
    })
  }, [data])

  function toggleExpanded(id: string) {
    setExpandedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      saveExpanded(next)
      return next
    })
  }

  if (loading) {
    return (
      <div className="p-4 text-overlay-0 text-sm">Loading projects…</div>
    )
  }

  if (error !== null) {
    return (
      <div className="p-4 text-red text-sm">
        <p className="font-medium">Failed to load projects</p>
        <p className="text-xs mt-1 text-overlay-1">{error}</p>
        <button
          onClick={load}
          className="mt-2 text-xs px-3 py-1 rounded bg-surface-0 text-text hover:bg-surface-1 transition-colors"
        >
          Retry
        </button>
      </div>
    )
  }

  if (data.projects.length === 0 && data.unmatched.length === 0) {
    return (
      <div className="space-y-3 p-4">
        <HostCard
          onAttach={onAttach}
          expanded={expandedIds.has(HOST_ID)}
          onToggle={() => toggleExpanded(HOST_ID)}
        />
        <div className="text-overlay-0 text-sm">No projects found.</div>
      </div>
    )
  }

  return (
    <div className="space-y-3 p-4">
      <HostCard
        onAttach={onAttach}
        expanded={expandedIds.has(HOST_ID)}
        onToggle={() => toggleExpanded(HOST_ID)}
      />
      {data.projects.map(project => (
        <ProjectCard
          key={project.encoded_path}
          project={project}
          expanded={expandedIds.has(project.encoded_path)}
          onToggle={() => toggleExpanded(project.encoded_path)}
          onAttach={onAttach}
          onRefresh={load}
        />
      ))}
      {data.unmatched.length > 0 && (
        <>
          <div className="text-xs text-overlay-1 uppercase tracking-wide px-4 font-semibold">Other</div>
          {data.unmatched.map(container => (
            <ContainerCard
              key={container.id}
              container={container}
              onRefresh={load}
              onAttach={onAttach}
              expanded={expandedIds.has(container.id)}
              onToggle={() => toggleExpanded(container.id)}
            />
          ))}
        </>
      )}
    </div>
  )
}
