import { useState, useCallback } from 'react'
import { type Container, createSession, destroySession, startContainer, stopContainer, destroyContainer } from '../api'
import { useConfirmAction } from '../lib/useConfirmAction'
import { SessionItem } from './SessionItem'

type ContainerCardProps = {
  readonly container: Container
  readonly onRefresh: () => void
  readonly onAttach: (containerId: string, containerName: string, sessionName: string) => void
  readonly expanded: boolean
  readonly onToggle: () => void
}

function stateColorClass(state: string): string {
  switch (state) {
    case 'running':
      return 'text-green'
    case 'stopped':
      return 'text-yellow'
    default:
      return 'text-overlay-0'
  }
}

export function ContainerCard({ container, onRefresh, onAttach, expanded, onToggle }: ContainerCardProps) {
  const [newSessionName, setNewSessionName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [actionLoading, setActionLoading] = useState(false)

  function showError(message: string) {
    setError(message)
    setTimeout(() => setError(null), 3000)
  }

  async function handleStart() {
    setActionLoading(true)
    try {
      await startContainer(container.id)
      onRefresh()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to start container')
    } finally {
      setActionLoading(false)
    }
  }

  async function handleStop() {
    setActionLoading(true)
    try {
      await stopContainer(container.id)
      onRefresh()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to stop container')
    } finally {
      setActionLoading(false)
    }
  }

  const destroyConfirm = useConfirmAction(
    useCallback(async () => {
      try {
        await destroyContainer(container.id)
        onRefresh()
      } catch (err) {
        showError(err instanceof Error ? err.message : 'failed to destroy container')
      }
    }, [container.id, onRefresh]),
  )

  async function handleCreateSession() {
    const name = newSessionName.trim()
    if (!name) return
    setCreating(true)
    try {
      await createSession(container.id, name)
      setNewSessionName('')
      onRefresh()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to create session')
    } finally {
      setCreating(false)
    }
  }

  async function handleDestroySession(name: string) {
    try {
      await destroySession(container.id, name)
      onRefresh()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to destroy session')
    }
  }

  const isRunning = container.state === 'running'

  return (
    <div className="w-full border border-surface-1 rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 bg-mantle hover:bg-surface-0 transition-colors text-left"
      >
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-text font-semibold truncate">{container.name}</span>
          <span className={`text-xs font-mono shrink-0 ${stateColorClass(container.state)}`}>
            {container.state}
          </span>
        </div>
        <span className="text-overlay-0 text-xs ml-2 shrink-0">
          {expanded ? '▲' : '▼'}
        </span>
      </button>

      {expanded && (
        <div className="px-4 py-3 bg-base space-y-3">
          {/* Metadata */}
          <div className="space-y-1 text-xs">
            <div className="flex gap-2">
              <span className="text-overlay-0 w-20 shrink-0">template</span>
              <span className="text-subtext-1 truncate">{container.template}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-overlay-0 w-20 shrink-0">path</span>
              <span className="text-subtext-1 truncate font-mono">{container.project_path}</span>
            </div>
          </div>

          {/* Container lifecycle actions */}
          <div className="flex items-center gap-2 mb-2">
            {container.state === 'running' ? (
              <>
                <button
                  onClick={handleStop}
                  disabled={actionLoading}
                  className="text-xs px-2 py-1 rounded bg-surface-1 text-text hover:bg-surface-2 disabled:opacity-40 transition-colors"
                >
                  Stop
                </button>
                <button
                  onClick={destroyConfirm.handleClick}
                  disabled={destroyConfirm.state === 'executing' || actionLoading}
                  className={`text-xs px-2 py-1 rounded transition-colors ${
                    destroyConfirm.state === 'confirming'
                      ? 'bg-red text-crust'
                      : 'bg-surface-1 text-red hover:bg-surface-2'
                  } disabled:opacity-40`}
                >
                  {destroyConfirm.state === 'executing' ? '…' : destroyConfirm.state === 'confirming' ? 'Confirm?' : 'Destroy'}
                </button>
              </>
            ) : (
              <button
                onClick={handleStart}
                disabled={actionLoading}
                className="text-xs px-2 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 disabled:opacity-40 transition-opacity"
              >
                Start
              </button>
            )}
          </div>

          {/* Error message */}
          {error !== null && (
            <div className="text-xs text-red bg-surface-0 rounded px-3 py-2">
              {error}
            </div>
          )}

          {/* Sessions */}
          <div className="space-y-2">
            <span className="text-xs text-overlay-1 uppercase tracking-wide">Sessions</span>
            {container.sessions.length === 0 ? (
              <p className="text-xs text-overlay-0">No sessions</p>
            ) : (
              <div className="space-y-1">
                {container.sessions.map(session => (
                  <SessionItem
                    key={session.name}
                    containerId={container.id}
                    session={session}
                    onDestroy={handleDestroySession}
                    onAttach={(cId, sName) => onAttach(cId, container.name, sName)}
                  />
                ))}
              </div>
            )}
          </div>

          {/* New session form (only for running containers) */}
          {isRunning && (
            <div className="flex gap-2">
              <input
                type="text"
                value={newSessionName}
                onChange={e => setNewSessionName(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') handleCreateSession()
                }}
                placeholder="Session name"
                className="flex-1 min-w-0 text-sm bg-surface-0 border border-surface-1 rounded px-2 py-1 text-text placeholder:text-overlay-0 focus:outline-none focus:border-blue"
              />
              <button
                onClick={handleCreateSession}
                disabled={creating || newSessionName.trim() === ''}
                className="text-sm px-3 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 disabled:opacity-40 transition-opacity shrink-0"
              >
                {creating ? '…' : 'New Session'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
