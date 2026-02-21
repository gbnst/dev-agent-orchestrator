// handoffDetector.ts — Detects Claude Code plugin handoff patterns.
//
// Functional Core: pure function, no side effects.
//
// Claude Code plugins (e.g. ed3d-plan-and-execute) print instructions like:
//
// Variant A (with code fences):
//   (1) Copy this command now:
//   ```
//   /some-skill --with args
//   ```
//   (2) Clear your context:
//
// Variant B (bare, with blank line):
//   (1) Copy this command now:
//
//   /some-skill --with args
//
//   (2) Clear your context:
//
// This detector finds the most recent such pattern in the terminal buffer
// and offers one-click actions for /clear and the extracted command.

import type { Detector, DetectorResult } from '../smartActions'

// Matches "(1) Copy this command[now]:" followed by a /command, either:
//   - on the same line: (1) Copy this command: /foo
//   - on the next line (with optional blank lines / code fences): (1) Copy this command now:\n```\n/foo
const HANDOFF_PATTERN =
  /\(1\)\s*Copy this command(?:\s+now)?[:\s]+(?:`{3}\s*\n)?\s*(\/\S[^\n]*)/g

export const handoffDetector: Detector = {
  id: 'handoff',

  detect(text: string): DetectorResult | null {
    // Find all matches and take the last one (most recent in scrollback).
    const matches = [...text.matchAll(HANDOFF_PATTERN)]
    if (matches.length === 0) return null

    const lastMatch = matches[matches.length - 1]
    const command = lastMatch[1].trim()
    const matchIndex = lastMatch.index ?? 0

    // Verify that step (2) mentions /clear or clearing context after the match.
    const textAfterMatch = text.slice(matchIndex + lastMatch[0].length)
    if (!/\(2\)[^\n]*(?:clear|\/clear)/i.test(textAfterMatch)) return null

    return {
      detectorId: 'handoff',
      banner: 'Workflow handoff detected',
      detail: command,
      autoChain: true,
      actions: [
        { label: 'Clear & Run', input: '/clear' },
        { label: 'Run command', input: command },
      ],
    }
  },
}
