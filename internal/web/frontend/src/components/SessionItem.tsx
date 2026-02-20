import { type Session } from '../api'

type SessionItemProps = {
  readonly containerId: string
  readonly session: Session
  readonly onDestroy: (name: string) => Promise<void>
  readonly onAttach: (containerId: string, sessionName: string) => void
}

export function SessionItem({ containerId, session, onDestroy, onAttach }: SessionItemProps) {
  function handleAttach() {
    onAttach(containerId, session.name)
  }

  async function handleDestroy() {
    const confirmed = window.confirm(
      `Destroy session "${session.name}"? This cannot be undone.`,
    )
    if (!confirmed) return
    await onDestroy(session.name)
  }

  return (
    <div className="flex flex-wrap items-center gap-y-1 justify-between px-3 py-2 bg-surface-0 rounded">
      <div className="flex items-center gap-2 min-w-0">
        <span className="text-text font-mono text-sm truncate">{session.name}</span>
        <span className="text-subtext-0 text-xs shrink-0">
          {session.windows} {session.windows === 1 ? 'window' : 'windows'}
        </span>
        {session.attached && (
          <span className="text-green text-xs shrink-0">attached</span>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <button
          onClick={handleAttach}
          className="text-xs px-2 py-1 rounded bg-surface-1 text-blue hover:bg-surface-2 transition-colors"
        >
          Attach
        </button>
        <button
          onClick={handleDestroy}
          className="text-xs px-2 py-1 rounded bg-surface-1 text-red hover:bg-surface-2 transition-colors"
        >
          Destroy
        </button>
      </div>
    </div>
  )
}
