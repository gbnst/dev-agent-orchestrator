import { describe, expect, test } from 'vitest'
import { handoffDetector } from './handoffDetector'

describe('handoffDetector', () => {
  test('detects bare format with blank line separator', () => {
    const text = [
      'Some output above',
      '',
      '(1) Copy this command now:',
      '',
      '/execute-implementation-plan /Users/ed/project/docs/plan/',
      '',
      '(2) Clear your context:',
      '',
      '/clear',
      '',
      '(3) Paste and run the copied command.',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.detectorId).toBe('handoff')
    expect(result!.banner).toBe('Workflow handoff detected')
    expect(result!.actions).toEqual([
      { label: '(1) /clear', input: '/clear' },
      { label: '(2) Run command', input: '/execute-implementation-plan /Users/ed/project/docs/plan/' },
    ])
  })

  test('detects code fence format', () => {
    const text = [
      '(1) Copy this command now:',
      '```',
      '/ed3d-plan-and-execute:start-implementation-plan @docs/design-plans/plan.md .',
      '```',
      '',
      '(2) Clear your context:',
      '```',
      '/clear',
      '```',
      '',
      '(3) Paste and run the copied command.',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe(
      '/ed3d-plan-and-execute:start-implementation-plan @docs/design-plans/plan.md .',
    )
  })

  test('detects indented format (command on next line)', () => {
    const text = [
      '(1) Copy this command now:',
      '    /execute-implementation-plan docs/plan.md',
      '(2) Clear your context by running /clear',
      '(3) Paste and run the command above',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe('/execute-implementation-plan docs/plan.md')
  })

  test('detects command on same line without "now"', () => {
    const text = [
      '● (1) Copy this command: /ed3d-plan-and-execute:start-implementation-plan @docs/plan.md',
      '',
      '  (2) Run /clear to clear your context.',
      '',
      '  (3) Paste and run the copied command.',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe(
      '/ed3d-plan-and-execute:start-implementation-plan @docs/plan.md',
    )
  })

  test('returns null when no handoff pattern present', () => {
    const text = 'Just some normal terminal output\n$ ls -la\nfile.txt'
    expect(handoffDetector.detect(text)).toBeNull()
  })

  test('returns null when step (1) exists but step (2) is missing', () => {
    const text = [
      '(1) Copy this command now:',
      '    /some-command',
      'Some other text without step 2',
    ].join('\n')

    expect(handoffDetector.detect(text)).toBeNull()
  })

  test('uses the last match when multiple handoffs exist in scrollback', () => {
    const text = [
      '(1) Copy this command now:',
      '',
      '/old-command --first',
      '',
      '(2) Clear your context:',
      '',
      'More output...',
      '',
      '(1) Copy this command now:',
      '',
      '/new-command --latest',
      '',
      '(2) Clear your context:',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe('/new-command --latest')
  })

  test('trims whitespace from extracted command', () => {
    const text = [
      '(1) Copy this command now:',
      '     /skill-name arg1 arg2   ',
      '(2) Clear your context:',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe('/skill-name arg1 arg2')
  })

  test('handles varying whitespace between step number and text', () => {
    const text = [
      '(1)Copy this command now:',
      '  /compact-command',
      '(2)  Clear your context:',
    ].join('\n')

    const result = handoffDetector.detect(text)

    expect(result).not.toBeNull()
    expect(result!.actions[1].input).toBe('/compact-command')
  })

  test('ignores incomplete last match when step (2) only follows earlier match', () => {
    const text = [
      '(1) Copy this command now:',
      '    /first-command',
      '(2) Clear your context:',
      '',
      '(1) Copy this command now:',
      '    /second-command',
      'No step 2 here',
    ].join('\n')

    expect(handoffDetector.detect(text)).toBeNull()
  })
})
