// ScrollButtons.tsx — Floating scroll buttons for mobile terminal scrollback.
//
// xterm.js 6.x has no native touch scroll support, so we provide ▲/▼ buttons
// that send mouse wheel escape sequences to the PTY. Since all templates run
// tmux with `mouse on`, tmux interprets these as scroll events — entering
// copy-mode on scroll-up and exiting when scrolled back to the bottom.
//
// Hidden on desktop (md:hidden). Uses the same press-and-hold repeat pattern
// as the ExtraKeysBar arrow keys.

import { useCallback, useRef } from 'react'
import type { XTermHandle } from '../lib/smartActions'

const REPEAT_DELAY_MS = 400
const REPEAT_INTERVAL_MS = 80
const SCROLL_LINES = 3

// SGR mouse protocol escape sequences for wheel events.
// Format: CSI < button ; x ; y M
// Button 64 = wheel up, 65 = wheel down. Coordinates don't matter for tmux
// scroll — we use (1,1).
const WHEEL_UP = '\x1b[<64;1;1M'
const WHEEL_DOWN = '\x1b[<65;1;1M'

type ScrollButtonsProps = {
  readonly handles: Map<string, XTermHandle>
  readonly tabKey: string
}

export function ScrollButtons({ handles, tabKey }: ScrollButtonsProps) {
  const handlesRef = useRef(handles)
  handlesRef.current = handles
  const tabKeyRef = useRef(tabKey)
  tabKeyRef.current = tabKey

  const repeatState = useRef<{
    timeout: ReturnType<typeof setTimeout> | null
    interval: ReturnType<typeof setInterval> | null
  }>({ timeout: null, interval: null })

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

  function scroll(direction: 'up' | 'down') {
    const h = handlesRef.current.get(tabKeyRef.current)
    if (!h) return
    const seq = direction === 'up' ? WHEEL_UP : WHEEL_DOWN
    for (let i = 0; i < SCROLL_LINES; i++) {
      h.sendInput(seq)
    }
  }

  function handleTouchStart(direction: 'up' | 'down') {
    scroll(direction)
    stopRepeat()
    repeatState.current.timeout = setTimeout(() => {
      repeatState.current.interval = setInterval(() => {
        scroll(direction)
      }, REPEAT_INTERVAL_MS)
    }, REPEAT_DELAY_MS)
  }

  function handleTouchEnd(e: React.TouchEvent) {
    e.preventDefault()
    stopRepeat()
  }

  function handleClick(e: React.MouseEvent, direction: 'up' | 'down') {
    e.preventDefault()
    scroll(direction)
  }

  const buttonClass = `
    w-10 h-10 flex items-center justify-center
    bg-surface-0/80 backdrop-blur-sm text-subtext-1
    rounded-lg text-base select-none
    transition-colors active:bg-surface-1
  `

  const buttonStyle: React.CSSProperties = {
    touchAction: 'manipulation',
    WebkitTouchCallout: 'none',
    userSelect: 'none',
  }

  return (
    <div className="absolute top-2 right-2 z-50 md:hidden flex flex-col gap-1">
      <button
        type="button"
        className={buttonClass}
        style={buttonStyle}
        onTouchStart={() => handleTouchStart('up')}
        onTouchEnd={handleTouchEnd}
        onTouchCancel={() => stopRepeat()}
        onClick={e => handleClick(e, 'up')}
      >
        ▲
      </button>
      <button
        type="button"
        className={buttonClass}
        style={buttonStyle}
        onTouchStart={() => handleTouchStart('down')}
        onTouchEnd={handleTouchEnd}
        onTouchCancel={() => stopRepeat()}
        onClick={e => handleClick(e, 'down')}
      >
        ▼
      </button>
    </div>
  )
}
