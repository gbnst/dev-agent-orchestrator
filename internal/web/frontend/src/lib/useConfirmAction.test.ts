import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useConfirmAction } from './useConfirmAction'

describe('useConfirmAction', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('initial state is idle', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action))
    expect(result.current.state).toBe('idle')
  })

  it('first click transitions to confirming', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action))

    act(() => result.current.handleClick())
    expect(result.current.state).toBe('confirming')
    expect(action).not.toHaveBeenCalled()
  })

  it('second click executes action and returns to idle', async () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action))

    act(() => result.current.handleClick()) // → confirming
    expect(result.current.state).toBe('confirming')

    await act(async () => result.current.handleClick()) // → executing → idle
    expect(action).toHaveBeenCalledOnce()
    expect(result.current.state).toBe('idle')
  })

  it('auto-resets to idle after timeout', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action))

    act(() => result.current.handleClick()) // → confirming
    expect(result.current.state).toBe('confirming')

    act(() => vi.advanceTimersByTime(3000))
    expect(result.current.state).toBe('idle')
    expect(action).not.toHaveBeenCalled()
  })

  it('reset() cancels confirmation', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action))

    act(() => result.current.handleClick()) // → confirming
    expect(result.current.state).toBe('confirming')

    act(() => result.current.reset())
    expect(result.current.state).toBe('idle')
  })

  it('click during executing state is ignored', async () => {
    let resolveAction: (() => void) | null = null
    const action = vi.fn(
      () =>
        new Promise<void>(resolve => {
          resolveAction = resolve
        }),
    )
    const { result } = renderHook(() => useConfirmAction(action))

    act(() => result.current.handleClick()) // → confirming
    await act(async () => result.current.handleClick()) // → executing

    expect(result.current.state).toBe('executing')
    expect(action).toHaveBeenCalledOnce()

    // Try to click while executing (should be ignored)
    act(() => result.current.handleClick())
    expect(action).toHaveBeenCalledOnce() // Still just once

    // Resolve the action to finish
    await act(async () => {
      if (resolveAction) {
        resolveAction()
      }
    })

    // State should be idle now
    expect(result.current.state).toBe('idle')
  })

  it('uses custom timeout value', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useConfirmAction(action, 1000))

    act(() => result.current.handleClick()) // → confirming
    expect(result.current.state).toBe('confirming')

    act(() => vi.advanceTimersByTime(500))
    expect(result.current.state).toBe('confirming')

    act(() => vi.advanceTimersByTime(500))
    expect(result.current.state).toBe('idle')
  })


  it('clears timer on unmount', () => {
    const action = vi.fn().mockResolvedValue(undefined)
    const { result, unmount } = renderHook(() => useConfirmAction(action))

    const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout')

    // Start confirmation to create a timer
    act(() => result.current.handleClick())
    expect(result.current.state).toBe('confirming')

    // Now unmount should clear the timer
    unmount()

    expect(clearTimeoutSpy).toHaveBeenCalled()
    clearTimeoutSpy.mockRestore()
  })
})
