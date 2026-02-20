import { useEffect } from 'react'

export function useServerEvents(onRefresh: () => void) {
  useEffect(() => {
    const source = new EventSource('/api/events')
    source.addEventListener('refresh', onRefresh)
    return () => source.close()
  }, [onRefresh])
}
