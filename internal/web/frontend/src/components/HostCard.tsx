import { useState, useEffect, useCallback } from 'react'
import { type Session, fetchHostSessions, createHostSession, destroyHostSession } from '../api'
import { useServerEvents } from '../lib/useServerEvents'
import { SessionItem } from './SessionItem'

export const HOST_ID = '__host__'

type HostCardProps = {
  readonly onAttach: (containerId: string, containerName: string, sessionName: string) => void
  readonly expanded: boolean
  readonly onToggle: () => void
}

export function HostCard({ onAttach, expanded, onToggle }: HostCardProps) {
  const [sessions, setSessions] = useState<Array<Session>>([])
  const [newSessionName, setNewSessionName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await fetchHostSessions()
      setSessions(data)
    } catch {
      // Host sessions may fail if tmux is not installed; silently degrade
      setSessions([])
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  useServerEvents(load)

  function showError(message: string) {
    setError(message)
    setTimeout(() => setError(null), 3000)
  }

  async function handleCreateSession() {
    const name = newSessionName.trim()
    if (!name) return
    setCreating(true)
    try {
      await createHostSession(name)
      setNewSessionName('')
      load()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to create session')
    } finally {
      setCreating(false)
    }
  }

  async function handleDestroySession(name: string) {
    try {
      await destroyHostSession(name)
      load()
    } catch (err) {
      showError(err instanceof Error ? err.message : 'failed to destroy session')
    }
  }

  return (
    <div className="w-full border border-surface-1 rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 bg-mantle hover:bg-surface-0 transition-colors text-left"
      >
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-text font-semibold truncate">Host</span>
          <span className="text-xs font-mono shrink-0 text-green">local</span>
        </div>
        <span className="text-overlay-0 text-xs ml-2 shrink-0">
          {expanded ? '▲' : '▼'}
        </span>
      </button>

      {expanded && (
        <div className="px-4 py-3 bg-base space-y-3">
          {/* Error message */}
          {error !== null && (
            <div className="text-xs text-red bg-surface-0 rounded px-3 py-2">
              {error}
            </div>
          )}

          {/* Sessions */}
          <div className="space-y-2">
            <span className="text-xs text-overlay-1 uppercase tracking-wide">Sessions</span>
            {sessions.length === 0 ? (
              <p className="text-xs text-overlay-0">No sessions</p>
            ) : (
              <div className="space-y-1">
                {sessions.map(session => (
                  <SessionItem
                    key={session.name}
                    containerId={HOST_ID}
                    session={session}
                    onDestroy={handleDestroySession}
                    onAttach={(_cId, sName) => onAttach(HOST_ID, 'Host', sName)}
                  />
                ))}
              </div>
            )}
          </div>

          {/* New session form */}
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
        </div>
      )}
    </div>
  )
}
