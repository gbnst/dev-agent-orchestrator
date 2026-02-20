// smartActions.ts — Type definitions for the terminal smart actions system.
//
// Functional Core: pure data types, no side effects.
//
// Smart actions detect patterns in terminal output and offer one-click
// actions. The system is pluggable: add new Detector implementations to
// detect new patterns.

/** A single action button displayed in the overlay. */
export type SmartAction = {
  readonly label: string
  readonly input: string
}

/** Result returned by a detector when a pattern is found. */
export type DetectorResult = {
  readonly detectorId: string
  readonly banner: string
  readonly actions: ReadonlyArray<SmartAction>
}

/** A detector scans terminal text for a specific pattern. */
export type Detector = {
  readonly id: string
  detect: (text: string) => DetectorResult | null
}

/** Handle exposed by XTerm for sending input and reading buffer text. */
export type XTermHandle = {
  sendInput: (text: string) => void
  getBufferText: () => string
}

/**
 * Type text into the terminal then press Enter, with a small delay between
 * the text and the Enter keystroke. This prevents Claude Code's autocomplete
 * from intercepting the Enter when all bytes arrive in a single PTY write.
 */
export function typeAndSubmit(handle: XTermHandle, text: string): void {
  handle.sendInput(text)
  setTimeout(() => handle.sendInput('\r'), 100)
}
