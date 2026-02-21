// extraKeys.ts — Pure functions for terminal escape sequences.
//
// Functional Core: no side effects, no state. Maps virtual key names
// to the raw byte sequences expected by terminal applications.

/** Virtual keys available in the extra keys bar. */
export type ExtraKey = 'esc' | 'tab' | 'ctrl' | 'alt' | 'up' | 'down' | 'left' | 'right'

/** Arrow keys subset for modifier combinations. */
export type ArrowKey = 'up' | 'down' | 'left' | 'right'

const ARROW_CODES: Record<ArrowKey, string> = {
  up: 'A',
  down: 'B',
  right: 'C',
  left: 'D',
}

/** Returns the raw escape sequence for a key press (no modifiers). */
export function getKeySequence(key: ExtraKey): string | null {
  switch (key) {
    case 'esc': return '\x1b'
    case 'tab': return '\t'
    case 'up': return `\x1b[A`
    case 'down': return `\x1b[B`
    case 'right': return `\x1b[C`
    case 'left': return `\x1b[D`
    case 'ctrl':
    case 'alt':
      return null // modifiers don't produce sequences on their own
  }
}

/**
 * Returns CSI sequence for an arrow key with modifiers.
 * Modifier param follows xterm convention: 1 + (shift?1:0) + (alt?2:0) + (ctrl?4:0)
 */
export function getModifiedArrowSequence(arrow: ArrowKey, ctrl: boolean, alt: boolean): string {
  const modifier = 1 + (alt ? 2 : 0) + (ctrl ? 4 : 0)
  if (modifier === 1) {
    return `\x1b[${ARROW_CODES[arrow]}`
  }
  return `\x1b[1;${modifier}${ARROW_CODES[arrow]}`
}

/** Applies Ctrl modifier to a character (a-z → 0x01-0x1a). */
export function ctrlModify(char: string): string {
  const code = char.toLowerCase().charCodeAt(0)
  if (code >= 0x61 && code <= 0x7a) {
    return String.fromCharCode(code & 0x1f)
  }
  return char
}

/** Applies Alt modifier to a character (ESC prefix). */
export function altModify(char: string): string {
  return `\x1b${char}`
}

/** Checks whether a key is an arrow key. */
export function isArrowKey(key: ExtraKey): key is ArrowKey {
  return key === 'up' || key === 'down' || key === 'left' || key === 'right'
}
