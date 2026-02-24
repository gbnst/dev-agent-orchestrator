import { useState, useCallback, useRef, useEffect } from 'react'

type ConfirmState = 'idle' | 'confirming' | 'executing'

interface UseConfirmActionResult {
  state: ConfirmState
  handleClick: () => void
  reset: () => void
}

export function useConfirmAction(
  action: () => Promise<void>,
  timeoutMs = 3000,
): UseConfirmActionResult {
  const [state, setState] = useState<ConfirmState>('idle')
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Auto-reset confirmation after timeout
  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [])

  const reset = useCallback(() => {
    setState('idle')
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const handleClick = useCallback(() => {
    if (state === 'idle') {
      setState('confirming')
      timerRef.current = setTimeout(() => {
        setState('idle')
        timerRef.current = null
      }, timeoutMs)
    } else if (state === 'confirming') {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
        timerRef.current = null
      }
      setState('executing')
      action().finally(() => setState('idle'))
    }
  }, [state, action, timeoutMs])

  return { state, handleClick, reset }
}
