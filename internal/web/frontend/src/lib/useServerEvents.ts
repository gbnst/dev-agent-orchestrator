import { useEffect, useRef } from 'react'

export function useServerEvents(onRefresh: () => void) {
  const onRefreshRef = useRef(onRefresh)
  onRefreshRef.current = onRefresh

  useEffect(() => {
    const source = new EventSource('/api/events')
    source.addEventListener('refresh', () => onRefreshRef.current())
    return () => source.close()
  }, [])
}
