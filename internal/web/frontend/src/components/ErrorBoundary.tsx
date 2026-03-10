import { Component, type ErrorInfo, type ReactNode } from 'react'

type Props = {
  readonly children: ReactNode
}

type State = {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled render error:', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center h-screen bg-base text-text p-8">
          <h1 className="text-xl font-semibold mb-2">Something went wrong</h1>
          <p className="text-subtext-0 mb-4 text-sm font-mono max-w-lg break-all">
            {this.state.error?.message}
          </p>
          <button
            type="button"
            className="px-4 py-2 bg-surface-0 text-text rounded hover:bg-surface-1 transition-colors"
            onClick={() => this.setState({ hasError: false, error: null })}
          >
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
