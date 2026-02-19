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

type XTermProps = {
  readonly containerId: string
  readonly sessionName: string
  readonly onDisconnect?: () => void
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

export function XTerm({ containerId, sessionName, onDisconnect }: XTermProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  // Keep onDisconnect in a ref so the useEffect does not need it as a
  // dependency. Including a prop callback in deps would cause the terminal
  // to tear down and rebuild on every parent render.
  const onDisconnectRef = useRef(onDisconnect)
  onDisconnectRef.current = onDisconnect

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
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)
    term.open(el)
    fitAddon.fit()

    const wsUrl = buildWsUrl(containerId, sessionName)
    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      // Send initial dimensions as a JSON text resize frame.
      const dims = fitAddon.proposeDimensions()
      if (dims) {
        ws.send(buildResizeMessage(dims.cols, dims.rows))
      }
    }

    ws.onmessage = (event: MessageEvent) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data))
      }
    }

    ws.onclose = () => {
      onDisconnectRef.current?.()
    }

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

    // ResizeObserver auto-fits terminal to container dimensions.
    const observer = new ResizeObserver(() => {
      fitAddon.fit()
    })
    observer.observe(el)

    return () => {
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
