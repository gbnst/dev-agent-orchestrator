// TerminalTabs.tsx — Responsive tab bar for open terminal sessions.
//
// pattern: Functional Core / Imperative Shell
//   - Pure: Tab type, TerminalTabsProps type
//   - Impure: none (pure render component, no side effects)
//
// Mobile-first:
//   - Below 768px (md:hidden): dropdown <select> with close button
//   - 768px and above (hidden md:flex): horizontal tab bar with X close buttons

type Tab = {
  readonly containerId: string
  readonly containerName: string
  readonly sessionName: string
  readonly key: string // unique: `${containerId}:${sessionName}`
}

type TerminalTabsProps = {
  readonly tabs: Array<Tab>
  readonly activeKey: string
  readonly onSelect: (key: string) => void
  readonly onClose: (key: string) => void
}

export type { Tab }

export function TerminalTabs({ tabs, activeKey, onSelect, onClose }: TerminalTabsProps) {
  return (
    <>
      {/* Mobile: dropdown selector (below 768px) */}
      <div className="flex items-center gap-2 bg-mantle px-2 py-1 md:hidden">
        <select
          value={activeKey}
          onChange={e => onSelect(e.target.value)}
          className="flex-1 min-w-0 text-sm bg-surface-0 border border-surface-1 rounded px-2 py-1 text-text focus:outline-none focus:border-blue"
        >
          {tabs.map(tab => (
            <option key={tab.key} value={tab.key}>
              {tab.containerName} / {tab.sessionName}
            </option>
          ))}
        </select>
        <button
          onClick={() => onClose(activeKey)}
          className="text-overlay-0 hover:text-red text-sm px-2 py-1 shrink-0 transition-colors"
          aria-label="Close tab"
        >
          ✕
        </button>
      </div>

      {/* Desktop: horizontal tab bar (768px and above) */}
      <div className="hidden md:flex items-center gap-1 bg-mantle px-2 py-1 overflow-x-auto">
        {tabs.map(tab => {
          const isActive = tab.key === activeKey
          return (
            <div
              key={tab.key}
              className={`flex items-center gap-1 px-3 py-1 rounded text-sm shrink-0 ${
                isActive
                  ? 'bg-surface-0 text-text'
                  : 'text-subtext-0 hover:text-text'
              }`}
            >
              <button
                onClick={() => onSelect(tab.key)}
                className="max-w-xs truncate"
              >
                {tab.containerName} / {tab.sessionName}
              </button>
              <button
                onClick={() => onClose(tab.key)}
                className="text-overlay-0 hover:text-red ml-1 transition-colors"
                aria-label={`Close ${tab.containerName} / ${tab.sessionName}`}
              >
                ✕
              </button>
            </div>
          )
        })}
      </div>
    </>
  )
}
