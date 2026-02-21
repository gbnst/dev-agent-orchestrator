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

// Matches "(1) Copy this command[now]:" header line.
const STEP1_PATTERN = /\(1\)\s*Copy this command(?:\s+now)?[:\s]/g

// Matches "(2) ... clear ..." line that confirms the handoff.
const STEP2_PATTERN = /\(2\)[^\n]*(?:clear|\/clear)/i

export const handoffDetector: Detector = {
  id: 'handoff',

  detect(text: string): DetectorResult | null {
    // Find all step (1) matches and take the last one (most recent in scrollback).
    const step1Matches = [...text.matchAll(STEP1_PATTERN)]
    if (step1Matches.length === 0) return null

    const lastMatch = step1Matches[step1Matches.length - 1]
    const afterStep1 = text.slice((lastMatch.index ?? 0) + lastMatch[0].length)

    // Verify that step (2) follows.
    const step2Match = STEP2_PATTERN.exec(afterStep1)
    if (!step2Match) return null

    // Extract everything between step (1) header and step (2).
    const between = afterStep1.slice(0, step2Match.index)

    // Split into lines, strip code fences. Keep raw text for joining.
    const rawLines = between.split('\n').map(l => l.replace(/`{3}/g, ''))
    const trimmedLines = rawLines.map(l => l.trim())
    const cmdStartIdx = trimmedLines.findIndex(l => l.startsWith('/'))
    if (cmdStartIdx === -1) return null

    // Collect the command line plus any continuation lines.
    // Continuation lines: non-empty when trimmed, don't start with '('
    // (parenthetical notes), don't start with '/' (next command).
    const rawParts = [rawLines[cmdStartIdx]]
    for (let i = cmdStartIdx + 1; i < trimmedLines.length; i++) {
      const t = trimmedLines[i]
      if (t === '' || t.startsWith('(') || t.startsWith('/')) break
      rawParts.push(rawLines[i])
    }

    // Detect the leading indent of the command line to strip it from all parts.
    const indent = rawParts[0].match(/^(\s*)/)?.[1].length ?? 0

    // Join: strip indent from each raw line and concatenate directly.
    // The terminal wraps at column boundary, so no separator is needed —
    // if the break was at a space, the space is at the end of the prev line.
    const command = rawParts.map(l => l.slice(indent)).join('').trim()

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
