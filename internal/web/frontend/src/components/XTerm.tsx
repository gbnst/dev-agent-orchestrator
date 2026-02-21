// XTerm.tsx — React wrapper for xterm.js with manual WebSocket PTY bridge.
//
// Uses manual WebSocket (NOT AttachAddon) because we mix binary I/O frames
// (PTY data) with JSON text frames (resize control messages).
//
// Functional Core / Imperative Shell:
//   - Pure: buildWsUrl, buildResizeMessage
//   - Impure: useEffect (terminal lifecycle, WebSocket, ResizeObserver)

import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import type { XTermHandle } from '../lib/smartActions'

type XTermProps = {
  readonly containerId: string
  readonly sessionName: string
  readonly onDisconnect?: () => void
  readonly onReady?: (handle: XTermHandle) => void
  readonly onData?: () => void
  readonly customKeyHandler?: ((event: KeyboardEvent) => boolean) | null
}

// buildWsUrl constructs the WebSocket URL for the terminal endpoint.
function buildWsUrl(containerId: string, sessionName: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/containers/${containerId}/sessions/${sessionName}/terminal`
}

// buildResizeMessage returns a JSON text frame for terminal resize events.
function buildResizeMessage(cols: number, rows: number): string {
  return JSON.stringify({ type: 'resize', cols, rows })
}

// catppuccinMochaTheme is the Catppuccin Mocha colour palette for xterm.
const catppuccinMochaTheme = {
  background: '#1e1e2e',
  foreground: '#cdd6f4',
  cursor: '#f5e0dc',
  cursorAccent: '#1e1e2e',
  selectionBackground: '#45475a',
  black: '#45475a',
  red: '#f38ba8',
  green: '#a6e3a1',
  yellow: '#f9e2af',
  blue: '#89b4fa',
  magenta: '#cba6f7',
  cyan: '#94e2d5',
  white: '#bac2de',
  brightBlack: '#585b70',
  brightRed: '#f38ba8',
  brightGreen: '#a6e3a1',
  brightYellow: '#f9e2af',
  brightBlue: '#89b4fa',
  brightMagenta: '#cba6f7',
  brightCyan: '#94e2d5',
  brightWhite: '#a6adc8',
}

export function XTerm({ containerId, sessionName, onDisconnect, onReady, onData, customKeyHandler }: XTermProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  // Keep callbacks in refs so the useEffect does not need them as
  // dependencies. Including prop callbacks in deps would cause the terminal
  // to tear down and rebuild on every parent render.
  const onDisconnectRef = useRef(onDisconnect)
  onDisconnectRef.current = onDisconnect
  const onReadyRef = useRef(onReady)
  onReadyRef.current = onReady
  const onDataRef = useRef(onData)
  onDataRef.current = onData
  const customKeyHandlerRef = useRef(customKeyHandler)
  customKeyHandlerRef.current = customKeyHandler

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    // Terminal must be constructed inside useEffect — xterm accesses browser
    // APIs (DOM) during construction.
    const term = new Terminal({
      theme: catppuccinMochaTheme,
      fontFamily: 'monospace',
      fontSize: 14,
      cursorBlink: true,
      macOptionClickForcesSelection: true,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)
    term.open(el)

    // Attach custom key handler for virtual modifier keys (extra keys bar).
    // Uses a ref wrapper so the handler always reads the latest callback.
    term.attachCustomKeyEventHandler((event: KeyboardEvent) => {
      const handler = customKeyHandlerRef.current
      return handler ? handler(event) : true
    })

    // Defer initial fit to the next frame so the browser has completed layout
    // and the container element has measurable dimensions. Mobile Safari does
    // not report correct sizes synchronously after open().
    requestAnimationFrame(() => {
      fitAddon.fit()
    })

    const wsUrl = buildWsUrl(containerId, sessionName)
    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'

    // Closures for smart actions: read terminal buffer and send input.
    function getBufferText(): string {
      const buf = term.buffer.active
      const totalRows = buf.length
      const startRow = Math.max(0, totalRows - 200)
      const lines: string[] = []
      for (let i = startRow; i < totalRows; i++) {
        const line = buf.getLine(i)
        if (line) lines.push(line.translateToString(true))
      }
      return lines.join('\n')
    }

    function sendInput(text: string): void {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(text))
      }
    }

    ws.addEventListener('open', () => {
      // Send initial dimensions as a JSON text resize frame.
      const dims = fitAddon.proposeDimensions()
      if (dims) {
        ws.send(buildResizeMessage(dims.cols, dims.rows))
      }
      onReadyRef.current?.({ sendInput, getBufferText })
    })

    ws.addEventListener('message', (event: MessageEvent) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data))
      }
      onDataRef.current?.()
    })

    ws.addEventListener('close', () => {
      onDisconnectRef.current?.()
    })

    // User keystrokes → binary WebSocket frame (raw PTY input).
    const dataDispose = term.onData((data: string) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data))
      }
    })

    // Terminal resize (triggered by fitAddon.fit()) → JSON text frame.
    const resizeDispose = term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(buildResizeMessage(cols, rows))
      }
    })

    // Prevent page scrolling when touching the terminal area. On iOS Safari,
    // touch-dragging inside the terminal can scroll the entire page (pushing
    // the header off-screen). We block touchmove default on the container to
    // prevent this. Terminal scrollback scrolling is not yet supported on
    // touch devices (xterm 6.x limitation).
    function onTouchMove(e: TouchEvent) {
      e.preventDefault()
    }
    el.addEventListener('touchmove', onTouchMove, { passive: false })

    // Auto-copy on selection: stash selected text eagerly (xterm clears
    // selections on incoming PTY data), then write to clipboard on mouseup
    // (user gesture required by the Clipboard API).
    let pendingSelection = ''
    const selectionDispose = term.onSelectionChange(() => {
      const text = term.getSelection()
      if (text) {
        pendingSelection = text
      }
    })
    function onMouseUp() {
      if (pendingSelection) {
        navigator.clipboard.writeText(pendingSelection).catch(() => {})
      }
      pendingSelection = ''
    }
    el.addEventListener('mouseup', onMouseUp)

    // ResizeObserver auto-fits terminal to container dimensions.
    const observer = new ResizeObserver(() => {
      fitAddon.fit()
    })
    observer.observe(el)

    return () => {
      el.removeEventListener('touchmove', onTouchMove)
      el.removeEventListener('mouseup', onMouseUp)
      selectionDispose.dispose()
      observer.disconnect()
      dataDispose.dispose()
      resizeDispose.dispose()
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
      term.dispose()
    }
  }, [containerId, sessionName])

  // The container div must use 100% width/height with overflow hidden so
  // FitAddon can calculate terminal dimensions correctly.
  return (
    <div
      ref={containerRef}
      style={{ width: '100%', height: '100%', overflow: 'hidden' }}
    />
  )
}
