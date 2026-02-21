// ExtraKeysBar.tsx — Button row for keys unavailable on mobile keyboards.
//
// Renders Esc, Tab, Ctrl, Alt, and arrow buttons between the tab bar and the
// terminal. Hidden on desktop (md:hidden). Ctrl/Alt are sticky toggles that
// light up when active.

import type { ExtraKey } from '../lib/extraKeys'
import type { ExtraKeysState } from '../lib/useExtraKeys'

type ButtonDef = {
  readonly key: ExtraKey
  readonly label: string
}

const BUTTONS: readonly ButtonDef[] = [
  { key: 'esc', label: 'Esc' },
  { key: 'tab', label: 'Tab' },
  { key: 'ctrl', label: 'Ctrl' },
  { key: 'alt', label: 'Alt' },
  { key: 'left', label: '←' },
  { key: 'up', label: '↑' },
  { key: 'down', label: '↓' },
  { key: 'right', label: '→' },
]

type ExtraKeysBarProps = {
  readonly state: ExtraKeysState
}

export function ExtraKeysBar({ state }: ExtraKeysBarProps) {
  const { ctrlActive, altActive, handleExtraKey, onPointerDown, onPointerUp } = state

  function isLit(key: ExtraKey): boolean {
    return (key === 'ctrl' && ctrlActive) || (key === 'alt' && altActive)
  }

  return (
    <div className="shrink-0 md:hidden flex bg-mantle border-b border-surface-1">
      {BUTTONS.map(({ key, label }) => (
        <button
          key={key}
          type="button"
          className={`
            flex-1 min-h-[44px] text-sm font-mono select-none
            transition-colors active:bg-surface-1
            ${isLit(key)
              ? 'bg-blue/30 text-blue'
              : 'bg-surface-0 text-subtext-1'
            }
          `}
          style={{
            touchAction: 'manipulation',
            WebkitTouchCallout: 'none',
            userSelect: 'none',
          }}
          onPointerDown={(e) => {
            e.preventDefault() // keep iOS keyboard open by not stealing focus
            handleExtraKey(key)
            onPointerDown(key)
          }}
          onPointerUp={onPointerUp}
          onPointerLeave={onPointerUp}
          onPointerCancel={onPointerUp}
        >
          {label}
        </button>
      ))}
    </div>
  )
}
