// pattern: Imperative Shell

package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FormField represents the currently focused form field.
type FormField int

const (
	FieldTemplate FormField = iota
	FieldProjectPath
	FieldContainerName
	fieldCount // Used for wrap-around
)

// Form state accessors for testing and view rendering.

// IsFormOpen returns true if the container creation form is open.
func (m Model) IsFormOpen() bool {
	return m.formOpen
}

// FormProjectPath returns the current project path input.
func (m Model) FormProjectPath() string {
	return m.formProjectPath
}

// FormContainerName returns the current container name input.
func (m Model) FormContainerName() string {
	return m.formContainerName
}

// FormTemplateIndex returns the currently selected template index.
func (m Model) FormTemplateIndex() int {
	return m.formTemplateIdx
}

// FormFocusedField returns the currently focused form field index.
func (m Model) FormFocusedField() int {
	return int(m.formFocusedField)
}

// FormError returns any validation error message.
func (m Model) FormError() string {
	return m.formError
}

// resetForm clears the form state.
func (m *Model) resetForm() {
	m.formOpen = false
	m.formTemplateIdx = 0
	m.formProjectPath = ""
	m.formContainerName = ""
	m.formFocusedField = FieldTemplate
	m.formError = ""

	// Clear submission progress state
	m.formSubmitting = false
	m.formTitlePulse = 0
	m.formStatusSteps = nil
	m.formCurrentStep = ""
	m.formCompleted = false
	m.formCompletedError = false
}

// openForm opens the creation form.
func (m *Model) openForm() {
	m.formOpen = true
	m.formTemplateIdx = 0
	m.formProjectPath = ""
	m.formContainerName = ""
	m.formFocusedField = FieldTemplate
	m.formError = ""

	// Check if templates are available
	if len(m.templates) == 0 {
		m.formError = "No templates available"
	}
}

// validateForm validates form inputs before submission.
func (m *Model) validateForm() bool {
	if len(m.formProjectPath) == 0 {
		m.formError = "Project path is required"
		return false
	}
	if len(m.templates) == 0 {
		m.formError = "No templates available"
		return false
	}
	m.formError = ""
	return true
}

// formTitlePulseMsg triggers the title pulse animation.
type formTitlePulseMsg struct{}

// startFormSubmission transitions the form to submitting state with spinners.
func (m *Model) startFormSubmission() tea.Cmd {
	m.formSubmitting = true
	m.formTitlePulse = 0
	m.formStatusSteps = nil
	m.formCurrentStep = ""
	m.formCompleted = false
	m.formCompletedError = false

	// Initialize status spinner (for current step)
	m.formStatusSpinner = spinner.New()
	m.formStatusSpinner.Spinner = spinner.MiniDot
	m.formStatusSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(m.styles.flavor.Teal().Hex))

	// Start both the spinner and the title pulse
	return tea.Batch(m.formStatusSpinner.Tick, tickTitlePulse())
}

// tickTitlePulse returns a command that ticks the title pulse every 200ms.
func tickTitlePulse() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return formTitlePulseMsg{}
	})
}

// addFormStatusStep appends a completed step to the form progress.
func (m *Model) addFormStatusStep(success bool, message string) {
	m.formStatusSteps = append(m.formStatusSteps, FormStatusStep{
		Success: success,
		Message: message,
	})
}

// setFormCurrentStep updates the current step being processed.
func (m *Model) setFormCurrentStep(message string) {
	m.formCurrentStep = message
}

// finishFormSubmission marks the form submission as complete.
func (m *Model) finishFormSubmission(success bool) tea.Cmd {
	m.formSubmitting = false
	m.formCompleted = true
	m.formCompletedError = !success
	m.formCurrentStep = ""

	// No auto-close - user must press Enter or Esc
	return nil
}

// IsFormSubmitting returns true if the form is currently submitting.
func (m Model) IsFormSubmitting() bool {
	return m.formSubmitting
}

// IsFormCompleted returns true if form submission has finished.
func (m Model) IsFormCompleted() bool {
	return m.formCompleted
}
