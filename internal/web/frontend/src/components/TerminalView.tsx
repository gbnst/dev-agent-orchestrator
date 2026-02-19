// TerminalView.tsx — Container for tab bar + active terminal.
//
// Keeps all terminal instances mounted (CSS show/hide) to preserve session
// state when switching between tabs. Inactive terminals are hidden via
// display:none rather than unmounted.
//
// Layout: full-height flexbox column.
//   - Tab bar at top (fixed height)
//   - Terminal fills remaining space (flex-1 min-h-0 overflow-hidden)

import { lazy, Suspense, useState } from 'react'
import { TerminalTabs, type Tab } from './TerminalTabs'

// Lazy-load XTerm so xterm.js is only bundled in a separate chunk and
// downloaded when the user first opens a terminal view.
const XTerm = lazy(() => import('./XTerm').then(m => ({ default: m.XTerm })))

type TerminalViewProps = {
  readonly tabs: Array<Tab>
  readonly onTabsChange: (tabs: Array<Tab>) => void
  readonly onBack: () => void
}

export function TerminalView({ tabs, onTabsChange, onBack }: TerminalViewProps) {
  const [activeKey, setActiveKey] = useState<string>(() => tabs[0]?.key ?? '')

  function handleSelect(key: string) {
    setActiveKey(key)
  }

  function handleClose(key: string) {
    const remaining = tabs.filter(t => t.key !== key)
    onTabsChange(remaining)

    if (remaining.length === 0) {
      // No tabs left — go back to containers view.
      onBack()
      return
    }

    if (activeKey === key) {
      // Closed tab was active — activate the previous tab (or first).
      const closedIndex = tabs.findIndex(t => t.key === key)
      const nextIndex = Math.max(0, closedIndex - 1)
      setActiveKey(remaining[nextIndex]?.key ?? remaining[0].key)
    }
  }

  // Sync activeKey when tabs change from outside (e.g. new tab added).
  // If activeKey is no longer in the list, fall back to the first tab.
  const activeKeyIsValid = tabs.some(t => t.key === activeKey)
  const resolvedActiveKey = activeKeyIsValid ? activeKey : (tabs[0]?.key ?? '')

  return (
    <div className="flex flex-col h-full">
      {/* Back button + tab bar */}
      <div className="shrink-0 flex items-center bg-mantle border-b border-surface-1">
        <button
          onClick={onBack}
          className="text-subtext-0 hover:text-text text-sm px-3 py-2 shrink-0 transition-colors"
        >
          ← Containers
        </button>
        <div className="flex-1 min-w-0">
          <TerminalTabs
            tabs={tabs}
            activeKey={resolvedActiveKey}
            onSelect={handleSelect}
            onClose={handleClose}
          />
        </div>
      </div>

      {/* Terminal panels — all mounted, inactive hidden via display:none */}
      <div className="flex-1 min-h-0 overflow-hidden relative">
        <Suspense fallback={
          <div className="flex items-center justify-center h-full text-subtext-0 text-sm">
            Loading terminal…
          </div>
        }>
          {tabs.map(tab => (
            <div
              key={tab.key}
              style={{
                display: tab.key === resolvedActiveKey ? 'block' : 'none',
                width: '100%',
                height: '100%',
              }}
            >
              <XTerm
                containerId={tab.containerId}
                sessionName={tab.sessionName}
              />
            </div>
          ))}
        </Suspense>
      </div>
    </div>
  )
}
