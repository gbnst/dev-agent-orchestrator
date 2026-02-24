import { useCallback } from 'react'
import { type Session } from '../api'
import { useConfirmAction } from '../lib/useConfirmAction'

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

  const destroyAction = useCallback(() => onDestroy(session.name), [onDestroy, session.name])
  const destroyConfirm = useConfirmAction(destroyAction)

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
      </div>
    </div>
  )
}
