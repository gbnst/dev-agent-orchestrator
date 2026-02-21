// useExtraKeys.ts — Hook managing modifier state and key repeat for the extra keys bar.
//
// Imperative Shell: manages state, timers, and sends input to the terminal.

import { useCallback, useRef, useState } from 'react'
import type { XTermHandle } from './smartActions'
import {
  type ArrowKey,
  type ExtraKey,
  altModify,
  ctrlModify,
  getKeySequence,
  getModifiedArrowSequence,
  isArrowKey,
} from './extraKeys'

const REPEAT_DELAY_MS = 400
const REPEAT_INTERVAL_MS = 80

type RepeatState = {
  timeout: ReturnType<typeof setTimeout> | null
  interval: ReturnType<typeof setInterval> | null
}

export type ExtraKeysState = {
  ctrlActive: boolean
  altActive: boolean
  handleExtraKey: (key: ExtraKey) => void
  onPointerDown: (key: ExtraKey) => void
  onPointerUp: () => void
  customKeyHandler: ((event: KeyboardEvent) => boolean) | null
}

export function useExtraKeys(handle: XTermHandle | undefined): ExtraKeysState {
  const [ctrlActive, setCtrlActive] = useState(false)
  const [altActive, setAltActive] = useState(false)

  // Refs so the customKeyHandler closure always reads current state.
  const ctrlRef = useRef(false)
  const altRef = useRef(false)
  const handleRef = useRef(handle)
  handleRef.current = handle

  const repeatState = useRef<RepeatState>({ timeout: null, interval: null })

  function setCtrl(value: boolean) {
    ctrlRef.current = value
    setCtrlActive(value)
  }

  function setAlt(value: boolean) {
    altRef.current = value
    setAltActive(value)
  }

  function clearModifiers() {
    setCtrl(false)
    setAlt(false)
  }

  function sendSequenceForKey(key: ExtraKey) {
    const h = handleRef.current
    if (!h) return

    if (isArrowKey(key) && (ctrlRef.current || altRef.current)) {
      h.sendInput(getModifiedArrowSequence(key as ArrowKey, ctrlRef.current, altRef.current))
      clearModifiers()
      return
    }

    const seq = getKeySequence(key)
    if (seq !== null) {
      h.sendInput(seq)
    }

    if (key === 'esc') {
      clearModifiers()
    }
  }

  const handleExtraKey = useCallback((key: ExtraKey) => {
    if (key === 'ctrl') {
      const next = !ctrlRef.current
      setCtrl(next)
      return
    }
    if (key === 'alt') {
      const next = !altRef.current
      setAlt(next)
      return
    }
    sendSequenceForKey(key)
  }, [])

  const stopRepeat = useCallback(() => {
    const rs = repeatState.current
    if (rs.timeout !== null) {
      clearTimeout(rs.timeout)
      rs.timeout = null
    }
    if (rs.interval !== null) {
      clearInterval(rs.interval)
      rs.interval = null
    }
  }, [])

  const onPointerDown = useCallback((key: ExtraKey) => {
    if (!isArrowKey(key)) return
    stopRepeat()
    repeatState.current.timeout = setTimeout(() => {
      repeatState.current.interval = setInterval(() => {
        sendSequenceForKey(key)
      }, REPEAT_INTERVAL_MS)
    }, REPEAT_DELAY_MS)
  }, [stopRepeat])

  const onPointerUp = useCallback(() => {
    stopRepeat()
  }, [stopRepeat])

  // Custom key handler for xterm's attachCustomKeyEventHandler.
  // Intercepts real keyboard input when Ctrl/Alt modifiers are active.
  const customKeyHandler = useCallback((event: KeyboardEvent): boolean => {
    // Only intercept keydown, not keyup
    if (event.type !== 'keydown') return true

    const h = handleRef.current
    if (!h) return true

    const ctrl = ctrlRef.current
    const alt = altRef.current

    // No virtual modifiers active — let xterm handle normally
    if (!ctrl && !alt) return true

    // Ignore bare modifier key presses
    if (event.key === 'Control' || event.key === 'Alt' || event.key === 'Shift' || event.key === 'Meta') {
      return true
    }

    // Single printable character with virtual modifier
    if (event.key.length === 1) {
      let seq = event.key
      if (ctrl) seq = ctrlModify(seq)
      if (alt) seq = altModify(seq)
      h.sendInput(seq)
      clearModifiers()
      return false // suppress xterm default handling
    }

    return true
  }, [])

  return {
    ctrlActive,
    altActive,
    handleExtraKey,
    onPointerDown,
    onPointerUp,
    customKeyHandler: handle ? customKeyHandler : null,
  }
}
