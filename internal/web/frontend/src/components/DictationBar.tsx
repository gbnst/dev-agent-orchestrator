// DictationBar.tsx — Combined input bar + extra keys for mobile.
//
// Top row: text input + send button (handles iOS dictation correctly).
// Bottom row: Esc, Tab, Ctrl, Alt, arrows, paste (keys missing on mobile).
//
// Hidden on desktop (md:hidden). Sits below the terminal.

import { useRef, useState } from 'react'
import type { XTermHandle } from '../lib/smartActions'
import { typeAndSubmit } from '../lib/smartActions'
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

const touchStyle: React.CSSProperties = {
  touchAction: 'manipulation',
  WebkitTouchCallout: 'none',
  userSelect: 'none',
}

type DictationBarProps = {
  readonly handle: XTermHandle | undefined
  readonly extraKeys: ExtraKeysState
}

export function DictationBar({ handle, extraKeys }: DictationBarProps) {
  const [value, setValue] = useState('')
  const [confirmClear, setConfirmClear] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const clearTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { ctrlActive, altActive, handleExtraKey, handlePaste, onPointerDown, onPointerUp } = extraKeys

  function handleSubmit() {
    const text = value.trim()
    if (!text || !handle) return
    typeAndSubmit(handle, text)
    setValue('')
    inputRef.current?.focus()
  }

  function handleClear() {
    if (!handle) return
    if (!confirmClear) {
      setConfirmClear(true)
      if (clearTimeoutRef.current) clearTimeout(clearTimeoutRef.current)
      clearTimeoutRef.current = setTimeout(() => setConfirmClear(false), 3000)
      return
    }
    setConfirmClear(false)
    if (clearTimeoutRef.current) clearTimeout(clearTimeoutRef.current)
    typeAndSubmit(handle, '/clear')
  }

  function isLit(key: ExtraKey): boolean {
    return (key === 'ctrl' && ctrlActive) || (key === 'alt' && altActive)
  }

  return (
    <div className="shrink-0 md:hidden bg-mantle border-t border-surface-1">
      {/* Dictation input */}
      <div className="flex items-center gap-1.5 px-2 py-1">
        <button
          type="button"
          onClick={handleClear}
          disabled={!handle}
          className={`shrink-0 text-sm px-2.5 py-1 rounded transition-colors disabled:opacity-40 disabled:cursor-default ${
            confirmClear
              ? 'bg-red/20 text-red hover:bg-red/30'
              : 'bg-surface-0 text-subtext-1 hover:bg-surface-1'
          }`}
          style={touchStyle}
        >
          {confirmClear ? 'Clr?' : 'Clr'}
        </button>
        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter') {
              e.preventDefault()
              handleSubmit()
            }
          }}
          placeholder="Dictate or type…"
          className="flex-1 min-w-0 bg-surface-0 text-text text-sm rounded px-2 py-1 border border-surface-1 focus:border-blue/50 focus:outline-none placeholder:text-overlay-0"
        />
        <button
          type="button"
          onClick={handleSubmit}
          disabled={!value.trim() || !handle}
          className="shrink-0 bg-blue/20 text-blue hover:bg-blue/30 disabled:opacity-40 disabled:cursor-default text-sm px-2.5 py-1 rounded transition-colors"
          style={touchStyle}
        >
          Send
        </button>
      </div>
      {/* Extra keys */}
      <div className="flex">
        {BUTTONS.map(({ key, label }) => (
          <button
            key={key}
            type="button"
            className={`flex-1 min-h-[36px] text-xs font-mono select-none transition-colors active:bg-surface-1 ${
              isLit(key) ? 'bg-blue/30 text-blue' : 'bg-surface-0 text-subtext-1'
            }`}
            style={touchStyle}
            onPointerDown={(e) => {
              e.preventDefault()
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
        <button
          type="button"
          className="flex-1 min-h-[36px] text-xs font-mono select-none transition-colors active:bg-surface-1 bg-surface-0 text-subtext-1"
          style={touchStyle}
          onPointerDown={(e) => {
            e.preventDefault()
            handlePaste()
          }}
        >
          ⎘
        </button>
      </div>
    </div>
  )
}
