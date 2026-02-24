import { useState, useEffect, useCallback } from 'react'
import { type Container, fetchContainers } from '../api'
import { useServerEvents } from '../lib/useServerEvents'
import { ContainerCard } from './ContainerCard'
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
  const [containers, setContainers] = useState<Array<Container>>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(loadExpanded)

  const load = useCallback(async () => {
    try {
      const data = await fetchContainers()
      setContainers(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to load containers')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  useServerEvents(load)

  // Prune stale IDs whenever the container list changes.
  useEffect(() => {
    const validIds = new Set<string>([HOST_ID, ...containers.map(c => c.id)])
    setExpandedIds(prev => {
      const pruned = new Set([...prev].filter(id => validIds.has(id)))
      if (pruned.size !== prev.size) {
        saveExpanded(pruned)
      }
      return pruned.size !== prev.size ? pruned : prev
    })
  }, [containers])

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
      <div className="p-4 text-overlay-0 text-sm">Loading containers…</div>
    )
  }

  if (error !== null) {
    return (
      <div className="p-4 text-red text-sm">
        <p className="font-medium">Failed to load containers</p>
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

  if (containers.length === 0) {
    return (
      <div className="space-y-3 p-4">
        <HostCard
          onAttach={onAttach}
          expanded={expandedIds.has(HOST_ID)}
          onToggle={() => toggleExpanded(HOST_ID)}
        />
        <div className="text-overlay-0 text-sm">No containers found.</div>
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
      {containers.map(container => (
        <ContainerCard
          key={container.id}
          container={container}
          onRefresh={load}
          onAttach={onAttach}
          expanded={expandedIds.has(container.id)}
          onToggle={() => toggleExpanded(container.id)}
        />
      ))}
    </div>
  )
}
