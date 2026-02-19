import { useState, useEffect, useCallback } from 'react'
import { type Container, fetchContainers } from '../api'
import { ContainerCard } from './ContainerCard'

type ContainerTreeProps = {
  readonly onAttach: (containerId: string, containerName: string, sessionName: string) => void
}

export function ContainerTree({ onAttach }: ContainerTreeProps) {
  const [containers, setContainers] = useState<Array<Container>>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

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
      <div className="p-4 text-overlay-0 text-sm">No containers found.</div>
    )
  }

  return (
    <div className="space-y-3 p-4">
      {containers.map(container => (
        <ContainerCard
          key={container.id}
          container={container}
          onRefresh={load}
          onAttach={onAttach}
        />
      ))}
    </div>
  )
}
