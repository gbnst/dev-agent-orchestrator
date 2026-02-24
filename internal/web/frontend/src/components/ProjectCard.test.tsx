import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProjectCard } from './ProjectCard'
import * as api from '../api'

// Mock the api module
vi.mock('../api', () => ({
  startContainer: vi.fn(),
  stopContainer: vi.fn(),
  destroyContainer: vi.fn(),
  createWorktree: vi.fn(),
  deleteWorktree: vi.fn(),
  createSession: vi.fn(),
  destroySession: vi.fn(),
  startWorktreeContainer: vi.fn(),
}))

describe('ProjectCard', () => {
  const mockOnToggle = vi.fn()
  const mockOnAttach = vi.fn()
  const mockOnRefresh = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('Start button for containerless worktrees (AC3.1, AC3.2, AC3.3, AC3.4)', () => {
    it('AC3.1: Start button visible when worktree selected and container === null', async () => {
      const user = userEvent.setup()
      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'main',
            path: '/path/to/project',
            is_main: true,
            container: null,
          },
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the containerless worktree (feature)
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Start button should be visible
      const startButton = screen.getByRole('button', { name: /^Start$/ })
      expect(startButton).toBeInTheDocument()
    })

    it('AC3.2: Clicking Start shows spinner/progress and calls onRefresh', async () => {
      const user = userEvent.setup()
      let resolveApiCall: (() => void) | null = null
      const mockStartWorktreeContainer = vi.spyOn(api, 'startWorktreeContainer')
      mockStartWorktreeContainer.mockImplementation(
        () =>
          new Promise<void>(resolve => {
            resolveApiCall = resolve
          }),
      )

      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Click Start button
      const startButton = screen.getByRole('button', { name: /^Start$/ })
      await user.click(startButton)

      // Should show stable loading indicator while container is starting
      await waitFor(() => {
        expect(screen.getByText('Starting container…')).toBeInTheDocument()
      })

      // API should be called with correct args
      expect(mockStartWorktreeContainer).toHaveBeenCalledWith(
        'dGVzdC1wcm9qZWN0',
        'feature',
      )

      // Complete the API call
      if (resolveApiCall) {
        resolveApiCall()
      }

      // After completion, onRefresh should be called
      await waitFor(() => {
        expect(mockOnRefresh).toHaveBeenCalled()
      })

      // Button should return to normal text
      expect(startButton.textContent).toBe('Start')
    })

    it('AC3.3: API error displays in action bar', async () => {
      const user = userEvent.setup()
      const mockStartWorktreeContainer = vi.spyOn(api, 'startWorktreeContainer')
      mockStartWorktreeContainer.mockRejectedValue(
        new Error('devcontainer up failed'),
      )

      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Click Start button
      const startButton = screen.getByRole('button', { name: /^Start$/ })
      await user.click(startButton)

      // Error message should appear in action bar
      await waitFor(() => {
        expect(
          screen.getByText('devcontainer up failed'),
        ).toBeInTheDocument()
      })

      // Verify the error message is visible
      const errorText = screen.getByText('devcontainer up failed')
      expect(errorText).toBeInTheDocument()
    })

    it('AC3.4: Start button NOT shown for worktrees with running container', async () => {
      const user = userEvent.setup()
      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: {
              id: 'abc123',
              name: 'feature-container',
              state: 'running',
              template: 'default',
              project_path: '/path/to/project',
              remote_user: 'user',
              created_at: '2026-02-24T00:00:00Z',
              sessions: [],
            },
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree with container
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Start button should NOT be visible (Stop button should be instead)
      const startButtons = screen.queryAllByRole('button', { name: /^Start$/ })
      expect(startButtons.length).toBe(0)

      // Stop button should be visible instead
      const stopButton = screen.getByRole('button', { name: /^Stop$/ })
      expect(stopButton).toBeInTheDocument()
    })

    it('AC3.4: Start button NOT shown for worktrees with stopped container', async () => {
      const user = userEvent.setup()
      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: {
              id: 'abc123',
              name: 'feature-container',
              state: 'exited',
              template: 'default',
              project_path: '/path/to/project',
              remote_user: 'user',
              created_at: '2026-02-24T00:00:00Z',
              sessions: [],
            },
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree with stopped container
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Start button for containerless worktree should NOT be visible
      // (the stopped container has its own Start button in a different section)
      const containerlessStartButtons = screen.queryAllByRole('button', {
        name: /^Start$/,
      })
      // Should have exactly 1 (the stopped container's Start button, not the containerless one)
      expect(containerlessStartButtons.length).toBe(1)
    })

    it('AC3.4: Start button shown for main worktree with no container, but no Delete', async () => {
      const user = userEvent.setup()
      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'main',
            path: '/path/to/project',
            is_main: true,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on main worktree
      const mainRadio = screen.getByRole('radio', { name: /main/ })
      await user.click(mainRadio)

      // Start button should be visible for containerless main worktree
      const startButton = screen.getByRole('button', { name: /^Start$/ })
      expect(startButton).toBeInTheDocument()

      // Delete Worktree should NOT be shown for main worktrees
      const deleteButtons = screen.queryAllByRole('button', { name: /Delete Worktree/ })
      expect(deleteButtons.length).toBe(0)
    })

    it('Shows loading indicator while starting, then restores Start button', async () => {
      const user = userEvent.setup()
      let resolveApiCall: (() => void) | null = null
      const mockStartWorktreeContainer = vi.spyOn(api, 'startWorktreeContainer')
      mockStartWorktreeContainer.mockImplementation(
        () =>
          new Promise<void>(resolve => {
            resolveApiCall = resolve
          }),
      )

      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Start button should be visible
      expect(screen.getByRole('button', { name: /^Start$/ })).toBeInTheDocument()

      // Click Start button
      await user.click(screen.getByRole('button', { name: /^Start$/ }))

      // Should show loading indicator instead of buttons
      await waitFor(() => {
        expect(screen.getByText('Starting container…')).toBeInTheDocument()
      })
      expect(screen.queryByRole('button', { name: /^Start$/ })).not.toBeInTheDocument()

      // Complete the API call
      if (resolveApiCall) {
        resolveApiCall()
      }

      // After completion, Start button should reappear
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /^Start$/ })).toBeInTheDocument()
      })
    })
  })

  describe('Action bar behavior', () => {
    it('Delete Worktree button still visible alongside Start button', async () => {
      const user = userEvent.setup()
      const project = {
        name: 'test-project',
        path: '/path/to/project',
        encoded_path: 'dGVzdC1wcm9qZWN0',
        has_makefile: false,
        worktrees: [
          {
            name: 'feature',
            path: '/path/to/project/worktrees/feature',
            is_main: false,
            container: null,
          },
        ],
      }

      render(
        <ProjectCard
          project={project}
          expanded={true}
          onToggle={mockOnToggle}
          onAttach={mockOnAttach}
          onRefresh={mockOnRefresh}
        />,
      )

      // Click on the worktree
      const featureRadio = screen.getByRole('radio', { name: /feature/ })
      await user.click(featureRadio)

      // Both Start and Delete Worktree buttons should be visible
      const startButton = screen.getByRole('button', { name: /^Start$/ })
      const deleteButton = screen.getByRole('button', { name: /Delete Worktree/ })

      expect(startButton).toBeInTheDocument()
      expect(deleteButton).toBeInTheDocument()
    })
  })
})
