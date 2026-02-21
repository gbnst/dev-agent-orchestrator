// SmartActionOverlay.tsx — Floating overlay for terminal smart actions.
//
// Functional Core / Imperative Shell:
//   - Pure: render logic (no side effects)
//   - Impure: none (callbacks are passed in)
//
// Renders inside each terminal's wrapper div (position: relative).
// Shows detected patterns with dismissible banners and action buttons.
//
// When a DetectorResult has `autoChain: true`, the overlay renders a single
// button (using the first action's label) that executes all actions in
// sequence via `onExecuteAll`. When `detail` is present, a read-only
// textarea shows the content below the buttons.

import type { DetectorResult, SmartAction } from '../lib/smartActions'

type SmartActionOverlayProps = {
  readonly results: ReadonlyArray<DetectorResult>
  readonly onDismiss: (detectorId: string) => void
  readonly onExecute: (action: SmartAction) => void
  readonly onExecuteAll: (actions: ReadonlyArray<SmartAction>) => void
}

export function SmartActionOverlay({ results, onDismiss, onExecute, onExecuteAll }: SmartActionOverlayProps) {
  if (results.length === 0) return null

  return (
    <div className="absolute top-2 left-2 z-10 flex flex-col gap-2 max-w-[80vw] md:hidden">
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
            {result.autoChain ? (
              <button
                onClick={() => onExecuteAll(result.actions)}
                className="bg-blue/20 text-blue hover:bg-blue/30 text-xs px-2.5 py-1 rounded transition-colors"
              >
                {result.actions[0]?.label}
              </button>
            ) : (
              result.actions.map(action => (
                <button
                  key={action.label}
                  onClick={() => onExecute(action)}
                  className="bg-blue/20 text-blue hover:bg-blue/30 text-xs px-2.5 py-1 rounded transition-colors"
                >
                  {action.label}
                </button>
              ))
            )}
          </div>
          {result.detail != null && (
            <textarea
              readOnly
              value={result.detail}
              rows={8}
              className="mt-2 w-full bg-mantle text-subtext-0 text-xs font-mono border border-surface-1 rounded px-2 py-1.5 resize-none focus:outline-none focus:border-blue/50"
            />
          )}
        </div>
      ))}
    </div>
  )
}
