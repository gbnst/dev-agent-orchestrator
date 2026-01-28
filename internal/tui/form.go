// pattern: Imperative Shell

package tui

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
