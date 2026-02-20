// SmartActionOverlay.tsx — Floating overlay for terminal smart actions.
//
// Functional Core / Imperative Shell:
//   - Pure: render logic (no side effects)
//   - Impure: none (callbacks are passed in)
//
// Renders inside each terminal's wrapper div (position: relative).
// Shows detected patterns with dismissible banners and action buttons.

import type { DetectorResult, SmartAction } from '../lib/smartActions'

type SmartActionOverlayProps = {
  readonly results: ReadonlyArray<DetectorResult>
  readonly onDismiss: (detectorId: string) => void
  readonly onExecute: (action: SmartAction) => void
}

export function SmartActionOverlay({ results, onDismiss, onExecute }: SmartActionOverlayProps) {
  if (results.length === 0) return null

  return (
    <div className="absolute top-2 right-2 z-10 flex flex-col gap-2 max-w-sm">
      {results.map(result => (
        <div
          key={result.detectorId}
          className="bg-surface-0/95 backdrop-blur border border-surface-1 rounded-lg px-3 py-2 shadow-lg"
        >
          <div className="flex items-center justify-between gap-2 mb-2">
            <span className="text-subtext-1 text-xs font-medium">
              {result.banner}
            </span>
            <button
              onClick={() => onDismiss(result.detectorId)}
              className="text-overlay-0 hover:text-red text-xs transition-colors shrink-0"
              aria-label="Dismiss"
            >
              ✕
            </button>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {result.actions.map(action => (
              <button
                key={action.label}
                onClick={() => onExecute(action)}
                className="bg-blue/20 text-blue hover:bg-blue/30 text-xs px-2.5 py-1 rounded transition-colors"
              >
                {action.label}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
