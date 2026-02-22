// useSmartActions.ts — Hook that scans terminal buffer for smart action patterns.
//
// Imperative Shell: manages debounce timers, state, and side effects.
//
// Scanning triggers:
//   1. Active tab changes (immediate scan)
//   2. Terminal data received (2s debounce after last data)
//
// Dismissed state is per-tab: dismissing a detector on one tab does not
// affect other tabs. Dismissed state persists for the tab's lifetime.

import { useCallback, useEffect, useRef, useState } from 'react'
import type { DetectorResult, SmartAction, XTermHandle } from './smartActions'
import { typeAndSubmit } from './smartActions'
import { detectors } from './detectors'

const DEBOUNCE_MS = 2000

type UseSmartActionsReturn = {
  readonly results: ReadonlyArray<DetectorResult>
  readonly dismiss: (detectorId: string) => void
  readonly execute: (action: SmartAction, tabKey: string) => void
  readonly executeAll: (actions: ReadonlyArray<SmartAction>, tabKey: string) => void
  readonly notifyDataReceived: (tabKey: string) => void
}

export function useSmartActions(
  activeTabKey: string,
  handles: ReadonlyMap<string, XTermHandle>,
): UseSmartActionsReturn {
  const [results, setResults] = useState<ReadonlyArray<DetectorResult>>([])
  const dismissedRef = useRef<Map<string, Set<string>>>(new Map())
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const runDetectors = useCallback(
    (tabKey: string) => {
      const handle = handles.get(tabKey)
      if (!handle) {
        setResults([])
        return
      }

      const text = handle.getBufferText()
      const dismissed = dismissedRef.current.get(tabKey) ?? new Set()

      const matched: DetectorResult[] = []
      for (const detector of detectors) {
        if (dismissed.has(detector.id)) continue
        const result = detector.detect(text)
        if (result) matched.push(result)
      }

      setResults(matched)
    },
    [handles],
  )

  // Scan when active tab changes.
  useEffect(() => {
    if (activeTabKey) runDetectors(activeTabKey)
    else setResults([])
  }, [activeTabKey, runDetectors])

  // Debounced scan on incoming terminal data.
  const notifyDataReceived = useCallback(
    (tabKey: string) => {
      // Only debounce for the active tab.
      if (tabKey !== activeTabKey) return

      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        runDetectors(tabKey)
      }, DEBOUNCE_MS)
    },
    [activeTabKey, runDetectors],
  )

  // Cleanup debounce timer on unmount.
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [])

  const dismiss = useCallback(
    (detectorId: string) => {
      if (!dismissedRef.current.has(activeTabKey)) {
        dismissedRef.current.set(activeTabKey, new Set())
      }
      dismissedRef.current.get(activeTabKey)!.add(detectorId)
      setResults(prev => prev.filter(r => r.detectorId !== detectorId))
    },
    [activeTabKey],
  )

  const execute = useCallback(
    (action: SmartAction, tabKey: string) => {
      const handle = handles.get(tabKey)
      if (handle) typeAndSubmit(handle, action.input)
    },
    [handles],
  )

  const executeAll = useCallback(
    (actions: ReadonlyArray<SmartAction>, tabKey: string) => {
      const handle = handles.get(tabKey)
      if (!handle || actions.length === 0) return

      // Dismiss all results so the overlay clears immediately, revealing the
      // terminal underneath and preventing the overlay from intercepting further
      // touch events while the chained commands execute.
      setResults([])

      const CHAIN_DELAY_MS = 500
      actions.forEach((action, i) => {
        setTimeout(() => typeAndSubmit(handle, action.input), i * CHAIN_DELAY_MS)
      })
    },
    [handles],
  )

  return { results, dismiss, execute, executeAll, notifyDataReceived }
}
