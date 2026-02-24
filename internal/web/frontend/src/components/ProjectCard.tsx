import { useState, useCallback, useEffect } from 'react'
import { type Container, type ProjectResponse, startContainer, stopContainer, destroyContainer, createWorktree, deleteWorktree, createSession, destroySession, startWorktreeContainer } from '../api'
import { useConfirmAction } from '../lib/useConfirmAction'

type Selection =
  | { type: 'worktree'; worktreeName: string }
  | { type: 'session'; worktreeName: string; sessionName: string }
  | { type: 'new-worktree' }
  | null

type ProjectCardProps = {
  readonly project: ProjectResponse
  readonly expanded: boolean
  readonly onToggle: () => void
  readonly onAttach: (containerId: string, containerName: string, sessionName: string) => void
  readonly onRefresh: () => void
}

function worktreeStatusIndicator(container: Container | null): string {
  if (!container) return '◌'
  return container.state === 'running' ? '●' : '○'
}

function worktreeStatusColor(container: Container | null): string {
  if (!container) return 'text-overlay-0'
  return container.state === 'running' ? 'text-green' : 'text-yellow'
}

export function ProjectCard({ project, expanded, onToggle, onAttach, onRefresh }: ProjectCardProps) {
  const [selection, setSelection] = useState<Selection>(null)
  const [newWorktreeName, setNewWorktreeName] = useState('')
  const [creatingWorktree, setCreatingWorktree] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [startingWorktree, setStartingWorktree] = useState<string | null>(null)
  const [newSessionName, setNewSessionName] = useState('')
  const [creatingSession, setCreatingSession] = useState(false)

  function showActionError(message: string) {
    setActionError(message)
    setTimeout(() => setActionError(null), 3000)
  }

  // Create worktree handler
  async function handleCreateWorktree() {
    const name = newWorktreeName.trim()
    if (!name) return
    setCreatingWorktree(true)
    try {
      await createWorktree(project.encoded_path, name)
      setNewWorktreeName('')
      setSelection(null)
      onRefresh()
    } catch (err) {
      showActionError(err instanceof Error ? err.message : 'failed to create worktree')
    } finally {
      setCreatingWorktree(false)
    }
  }

  // Destroy container action with confirmation
  const destroyConfirm = useConfirmAction(
    useCallback(async () => {
      if (selection?.type !== 'worktree') return
      const worktree = project.worktrees.find(w => w.name === selection.worktreeName)
      if (!worktree?.container) return
      setActionLoading(true)
      try {
        await destroyContainer(worktree.container.id)
        onRefresh()
      } catch (err) {
        showActionError(err instanceof Error ? err.message : 'failed to destroy container')
      } finally {
        setActionLoading(false)
      }
    }, [selection, project.worktrees, onRefresh]),
  )

  // Delete worktree action with confirmation
  const deleteWorktreeConfirm = useConfirmAction(
    useCallback(async () => {
      if (selection?.type !== 'worktree') return
      try {
        await deleteWorktree(project.encoded_path, selection.worktreeName)
        setSelection(null)
        onRefresh()
      } catch (err) {
        showActionError(err instanceof Error ? err.message : 'failed to delete worktree')
      }
    }, [selection, project.encoded_path, onRefresh]),
  )

  // Destroy session action with confirmation
  const destroySessionConfirm = useConfirmAction(
    useCallback(async () => {
      if (selection?.type !== 'session') return
      const wt = project.worktrees.find(w => w.name === selection.worktreeName)
      if (!wt?.container) return
      try {
        await destroySession(wt.container.id, selection.sessionName)
        setSelection(null)
        onRefresh()
      } catch (err) {
        showActionError(err instanceof Error ? err.message : 'failed to destroy session')
      }
    }, [selection, project.worktrees, onRefresh]),
  )

  // Create session handler
  async function handleCreateSession(containerId: string) {
    const name = newSessionName.trim()
    if (!name) return
    setCreatingSession(true)
    try {
      await createSession(containerId, name)
      setNewSessionName('')
      onRefresh()
    } catch (err) {
      showActionError(err instanceof Error ? err.message : 'failed to create session')
    } finally {
      setCreatingSession(false)
    }
  }

  // Reset confirmation states when selection changes
  // eslint-disable-next-line react-hooks/exhaustive-deps -- only reset on selection change, not on confirm state changes
  useEffect(() => {
    destroyConfirm.reset()
    deleteWorktreeConfirm.reset()
    destroySessionConfirm.reset()
  }, [selection])

  function renderActionBar(): React.ReactNode {
    if (!selection) return null

    if (selection.type === 'worktree') {
      const worktree = project.worktrees.find(w => w.name === selection.worktreeName)
      if (!worktree) return null

      const isMain = worktree.is_main
      const container = worktree.container
      const isRunning = container?.state === 'running'

      // While a worktree container is being created, show a stable loading
      // indicator instead of switching to the container action buttons
      // (which would flash confusingly when SSE refresh delivers the new container)
      if (startingWorktree === worktree.name) {
        return (
          <div className="flex items-center gap-2">
            <span className="text-xs text-overlay-1">Starting container…</span>
          </div>
        )
      }

      return (
        <div className="flex flex-wrap gap-2">
          {isRunning && (
            <>
              <button
                onClick={async () => {
                  if (!container) return
                  setActionLoading(true)
                  try {
                    await stopContainer(container.id)
                    onRefresh()
                  } catch (err) {
                    showActionError(err instanceof Error ? err.message : 'failed to stop container')
                  } finally {
                    setActionLoading(false)
                  }
                }}
                disabled={actionLoading}
                className="text-xs px-3 py-1 rounded bg-surface-1 text-text hover:bg-surface-2 transition-colors disabled:opacity-40"
              >
                {actionLoading ? '…' : 'Stop'}
              </button>
              {isMain && (
                <button
                  onClick={destroyConfirm.handleClick}
                  disabled={destroyConfirm.state === 'executing'}
                  className={`text-xs px-2 py-1 rounded transition-colors ${
                    destroyConfirm.state === 'confirming'
                      ? 'bg-red text-crust'
                      : 'bg-surface-1 text-red hover:bg-surface-2'
                  } disabled:opacity-40`}
                >
                  {destroyConfirm.state === 'executing' ? '…' : destroyConfirm.state === 'confirming' ? 'Confirm?' : 'Destroy'}
                </button>
              )}
              {!isMain && (
                <button
                  onClick={deleteWorktreeConfirm.handleClick}
                  disabled={deleteWorktreeConfirm.state === 'executing'}
                  className={`text-xs px-2 py-1 rounded transition-colors ${
                    deleteWorktreeConfirm.state === 'confirming'
                      ? 'bg-red text-crust'
                      : 'bg-surface-1 text-red hover:bg-surface-2'
                  } disabled:opacity-40`}
                >
                  {deleteWorktreeConfirm.state === 'executing'
                    ? '…'
                    : deleteWorktreeConfirm.state === 'confirming'
                      ? 'Confirm delete? (removes container too)'
                      : 'Delete Worktree'}
                </button>
              )}
              {container && (
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={newSessionName}
                    onChange={e => setNewSessionName(e.target.value)}
                    onKeyDown={e => { if (e.key === 'Enter') handleCreateSession(container.id) }}
                    placeholder="Session name"
                    className="flex-1 min-w-0 text-sm bg-surface-0 border border-surface-1 rounded px-2 py-1 text-text placeholder:text-overlay-0 focus:outline-none focus:border-blue"
                  />
                  <button
                    onClick={() => handleCreateSession(container.id)}
                    disabled={creatingSession || newSessionName.trim() === ''}
                    className="text-sm px-3 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 disabled:opacity-40 transition-opacity shrink-0"
                  >
                    {creatingSession ? '…' : 'New Session'}
                  </button>
                </div>
              )}
            </>
          )}
          {!isRunning && container && (
            <>
              <button
                onClick={async () => {
                  setActionLoading(true)
                  try {
                    await startContainer(container.id)
                    onRefresh()
                  } catch (err) {
                    showActionError(err instanceof Error ? err.message : 'failed to start container')
                  } finally {
                    setActionLoading(false)
                  }
                }}
                disabled={actionLoading}
                className="text-xs px-3 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 transition-opacity disabled:opacity-40"
              >
                {actionLoading ? '…' : 'Start'}
              </button>
              {!isMain && (
                <button
                  onClick={deleteWorktreeConfirm.handleClick}
                  disabled={deleteWorktreeConfirm.state === 'executing'}
                  className={`text-xs px-2 py-1 rounded transition-colors ${
                    deleteWorktreeConfirm.state === 'confirming'
                      ? 'bg-red text-crust'
                      : 'bg-surface-1 text-red hover:bg-surface-2'
                  } disabled:opacity-40`}
                >
                  {deleteWorktreeConfirm.state === 'executing'
                    ? '…'
                    : deleteWorktreeConfirm.state === 'confirming'
                      ? 'Confirm delete? (removes container too)'
                      : 'Delete Worktree'}
                </button>
              )}
            </>
          )}
          {!container && startingWorktree !== worktree.name && (
            <>
              <button
                onClick={async () => {
                  setActionLoading(true)
                  setStartingWorktree(worktree.name)
                  try {
                    await startWorktreeContainer(project.encoded_path, worktree.name)
                    onRefresh()
                  } catch (err) {
                    showActionError(err instanceof Error ? err.message : 'failed to start container')
                  } finally {
                    setActionLoading(false)
                    setStartingWorktree(null)
                  }
                }}
                disabled={actionLoading}
                className="text-xs px-3 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 transition-opacity disabled:opacity-40"
              >
                {actionLoading ? 'Starting…' : 'Start'}
              </button>
              {!isMain && (
                <button
                  onClick={deleteWorktreeConfirm.handleClick}
                  disabled={deleteWorktreeConfirm.state === 'executing'}
                  className={`text-xs px-2 py-1 rounded transition-colors ${
                    deleteWorktreeConfirm.state === 'confirming'
                      ? 'bg-red text-crust'
                      : 'bg-surface-1 text-red hover:bg-surface-2'
                  } disabled:opacity-40`}
                >
                  {deleteWorktreeConfirm.state === 'executing'
                    ? '…'
                    : deleteWorktreeConfirm.state === 'confirming'
                      ? 'Confirm delete? (removes container too)'
                      : 'Delete Worktree'}
                </button>
              )}
            </>
          )}
        </div>
      )
    }

    if (selection.type === 'session') {
      const worktree = project.worktrees.find(w => w.name === selection.worktreeName)
      if (!worktree?.container) return null

      return (
        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              if (worktree.container) {
                onAttach(worktree.container.id, worktree.container.name, selection.sessionName)
              }
            }}
            className="text-xs px-2 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 transition-opacity"
          >
            Attach
          </button>
          <button
            onClick={destroySessionConfirm.handleClick}
            disabled={destroySessionConfirm.state === 'executing'}
            className={`text-xs px-2 py-1 rounded transition-colors ${
              destroySessionConfirm.state === 'confirming'
                ? 'bg-red text-crust'
                : 'bg-surface-1 text-red hover:bg-surface-2'
            } disabled:opacity-40`}
          >
            {destroySessionConfirm.state === 'executing' ? '…' : destroySessionConfirm.state === 'confirming' ? 'Confirm?' : 'Destroy'}
          </button>
        </div>
      )
    }

    if (selection.type === 'new-worktree') {
      return (
        <div className="flex gap-2">
          <input
            type="text"
            value={newWorktreeName}
            onChange={e => setNewWorktreeName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleCreateWorktree() }}
            placeholder="Branch name"
            className="flex-1 min-w-0 text-sm bg-surface-0 border border-surface-1 rounded px-2 py-1 text-text placeholder:text-overlay-0 focus:outline-none focus:border-blue"
          />
          <button
            onClick={handleCreateWorktree}
            disabled={creatingWorktree || newWorktreeName.trim() === ''}
            className="text-sm px-3 py-1 rounded bg-blue text-crust font-medium hover:opacity-80 disabled:opacity-40 transition-opacity shrink-0"
          >
            {creatingWorktree ? 'Creating…' : 'Create'}
          </button>
        </div>
      )
    }

    return null
  }

  const radioName = `project-${project.encoded_path}`
  const actionBar = renderActionBar()

  return (
    <div className="w-full border border-surface-1 rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 bg-mantle hover:bg-surface-0 transition-colors text-left"
      >
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-text font-semibold truncate">{project.name}</span>
          {project.has_makefile && (
            <span className="text-xs font-mono shrink-0 text-blue">makefile</span>
          )}
        </div>
        <span className="text-overlay-0 text-xs ml-2 shrink-0">
          {expanded ? '▲' : '▼'}
        </span>
      </button>

      {expanded && (
        <div className="px-4 py-3 bg-base space-y-3">
          {/* Worktrees and sessions */}
          <div className="space-y-2">
            <span className="text-xs text-overlay-1 uppercase tracking-wide">Worktrees</span>
            {project.worktrees.length === 0 ? (
              <p className="text-xs text-overlay-0">No worktrees</p>
            ) : (
              <div className="space-y-1">
                {project.worktrees.map(worktree => (
                  <div key={worktree.name} className="space-y-1">
                    {/* Worktree row */}
                    <label className="flex items-center gap-2 px-2 py-1 hover:bg-surface-0 rounded cursor-pointer">
                      <input
                        type="radio"
                        name={radioName}
                        value={`worktree-${worktree.name}`}
                        checked={selection?.type === 'worktree' && selection.worktreeName === worktree.name}
                        onChange={() => setSelection({ type: 'worktree', worktreeName: worktree.name })}
                        className="shrink-0"
                      />
                      <span className={`text-sm font-mono shrink-0 ${worktreeStatusColor(worktree.container)}`}>
                        {worktreeStatusIndicator(worktree.container)}
                      </span>
                      <span className="text-sm text-text truncate">{worktree.name}</span>
                      {worktree.is_main && (
                        <span className="text-xs text-overlay-1 shrink-0">main</span>
                      )}
                    </label>

                    {/* Sessions nested under worktree */}
                    {worktree.container && worktree.container.sessions.length > 0 && (
                      <div className="ml-6 space-y-1">
                        {worktree.container.sessions.map(session => (
                          <label
                            key={session.name}
                            className="flex items-center gap-2 px-2 py-1 hover:bg-surface-0 rounded cursor-pointer"
                          >
                            <input
                              type="radio"
                              name={radioName}
                              value={`session-${worktree.name}-${session.name}`}
                              checked={selection?.type === 'session' && selection.worktreeName === worktree.name && selection.sessionName === session.name}
                              onChange={() => setSelection({ type: 'session', worktreeName: worktree.name, sessionName: session.name })}
                              className="shrink-0"
                            />
                            <span className="text-xs text-text font-mono truncate">{session.name}</span>
                            <span className="text-xs text-subtext-0 shrink-0">
                              {session.windows} {session.windows === 1 ? 'window' : 'windows'}
                            </span>
                            {session.attached && (
                              <span className="text-xs text-green shrink-0">attached</span>
                            )}
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* New worktree option */}
            <label className="flex items-center gap-2 px-2 py-1 hover:bg-surface-0 rounded cursor-pointer">
              <input
                type="radio"
                name={radioName}
                value="new-worktree"
                checked={selection?.type === 'new-worktree'}
                onChange={() => setSelection({ type: 'new-worktree' })}
                className="shrink-0"
              />
              <span className="text-sm text-text">New worktree</span>
            </label>
          </div>

          {/* Action bar */}
          {actionBar !== null && (
            <div className="pt-2 border-t border-surface-1 space-y-2">
              {actionError !== null && (
                <div className="text-xs text-red bg-surface-0 rounded px-3 py-2">
                  {actionError}
                </div>
              )}
              {actionBar}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
