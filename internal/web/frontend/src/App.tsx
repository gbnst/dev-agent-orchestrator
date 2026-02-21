// App.tsx — Top-level two-view SPA: containers list and terminal.
//
// Navigation uses component state — no router needed for this two-view SPA.
// Tabs remain mounted when navigating between views to preserve terminal state.

import { useCallback, useEffect, useState } from 'react'
import { ContainerTree } from './components/ContainerTree'
import { TerminalView } from './components/TerminalView'
import { type Tab } from './components/TerminalTabs'

function buildTabKey(containerId: string, sessionName: string): string {
  return `${containerId}:${sessionName}`
}

function App() {
  const [view, setView] = useState<'containers' | 'terminal'>('containers')
  const [tabs, setTabs] = useState<Array<Tab>>([])

  // Counteract iOS Safari's automatic scroll-into-view when the software
  // keyboard opens. Safari scrolls the page to reveal the focused textarea
  // (xterm's hidden input), pushing our fixed layout off-screen. Resetting
  // scroll on every visualViewport resize keeps the header pinned.
  useEffect(() => {
    const vv = window.visualViewport
    if (!vv) return

    function onResize() {
      window.scrollTo(0, 0)
    }
    vv.addEventListener('resize', onResize)
    return () => vv.removeEventListener('resize', onResize)
  }, [])

  // useCallback ensures handleAttach has a stable reference across renders so
  // ContainerTree does not re-render every time App re-renders.
  // setTabs and setView are stable (useState guarantees), so deps is empty.
  const handleAttach = useCallback((containerId: string, containerName: string, sessionName: string) => {
    const key = buildTabKey(containerId, sessionName)
    setTabs(prev => {
      // Add tab only if not already open.
      if (prev.some(t => t.key === key)) return prev
      return [...prev, { containerId, containerName, sessionName, key }]
    })
    setView('terminal')
  }, [])

  return (
    <div className="bg-base flex flex-col overflow-hidden fixed inset-0">
      {view === 'containers' && (
        <div className="w-full md:w-80 md:min-h-screen md:border-r md:border-surface-1 flex flex-col">
          <header className="px-4 py-3 border-b border-surface-1 bg-mantle shrink-0">
            <h1 className="text-text font-semibold text-base">devagent</h1>
          </header>
          <div className="flex-1 overflow-y-auto">
            <ContainerTree onAttach={handleAttach} />
          </div>
        </div>
      )}
      {view === 'terminal' && (
        <div className="flex-1 min-h-0">
          <TerminalView
            tabs={tabs}
            onTabsChange={setTabs}
            onBack={() => setView('containers')}
          />
        </div>
      )}
    </div>
  )
}

export default App
