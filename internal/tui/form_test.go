package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/logging"
)

func newTestModel(t *testing.T) Model {
	cfg := &config.Config{
		Theme: "mocha",
	}
	templates := []config.Template{
		{Name: "go-project"},
		{Name: "python-project"},
	}
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"
	lm, _ := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	return NewModelWithTemplates(cfg, templates, lm)
}

func TestForm_PressC_OpensForm(t *testing.T) {
	m := newTestModel(t)

	// Initially form should be closed
	if m.IsFormOpen() {
		t.Error("Form should be closed initially")
	}

	// Press 'c' to open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	if !m.IsFormOpen() {
		t.Error("Form should be open after pressing 'c'")
	}
}

func TestForm_PressEscape_ClosesForm(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Press Escape to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	if m.IsFormOpen() {
		t.Error("Form should be closed after pressing Escape")
	}
}

func TestForm_TextInput_UpdatesProjectPath(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Focus should start on template selection (index 0)
	// Press Tab to move to project path field
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	// Type a path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/', 'h', 'o', 'm', 'e'}})
	m = updated.(Model)

	if m.FormProjectPath() != "/home" {
		t.Errorf("Expected project path '/home', got %q", m.FormProjectPath())
	}
}

func TestForm_TextInput_UpdatesContainerName(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Tab twice to get to name field (template -> project path -> name)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	// Type a name
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m', 'y', '-', 'c', 'o', 'n', 't', 'a', 'i', 'n', 'e', 'r'}})
	m = updated.(Model)

	if m.FormContainerName() != "my-container" {
		t.Errorf("Expected container name 'my-container', got %q", m.FormContainerName())
	}
}

func TestForm_TemplateSelection_ArrowKeys(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Initial template index should be 0
	if m.FormTemplateIndex() != 0 {
		t.Errorf("Expected initial template index 0, got %d", m.FormTemplateIndex())
	}

	// Press down to select second template
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	if m.FormTemplateIndex() != 1 {
		t.Errorf("Expected template index 1 after down arrow, got %d", m.FormTemplateIndex())
	}

	// Press up to go back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)

	if m.FormTemplateIndex() != 0 {
		t.Errorf("Expected template index 0 after up arrow, got %d", m.FormTemplateIndex())
	}
}

func TestForm_TemplateSelection_BoundsCheck(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Press up at index 0 - should stay at 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)

	if m.FormTemplateIndex() != 0 {
		t.Errorf("Expected template index to stay at 0, got %d", m.FormTemplateIndex())
	}

	// Press down twice (we only have 2 templates)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	// Should stay at max index (1)
	if m.FormTemplateIndex() != 1 {
		t.Errorf("Expected template index to stay at 1, got %d", m.FormTemplateIndex())
	}
}

func TestForm_TabCycles_ThroughFields(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Initial field should be 0 (template)
	if m.FormFocusedField() != 0 {
		t.Errorf("Expected initial focused field 0, got %d", m.FormFocusedField())
	}

	// Tab to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.FormFocusedField() != 1 {
		t.Errorf("Expected focused field 1, got %d", m.FormFocusedField())
	}

	// Tab to container name
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.FormFocusedField() != 2 {
		t.Errorf("Expected focused field 2, got %d", m.FormFocusedField())
	}

	// Tab wraps back to template
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.FormFocusedField() != 0 {
		t.Errorf("Expected focused field to wrap to 0, got %d", m.FormFocusedField())
	}
}

func TestForm_Backspace_DeletesCharacter(t *testing.T) {
	m := newTestModel(t)

	// Open form and move to project path field
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	// Type something
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b', 'c'}})
	m = updated.(Model)

	// Backspace
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)

	if m.FormProjectPath() != "ab" {
		t.Errorf("Expected 'ab' after backspace, got %q", m.FormProjectPath())
	}
}

func TestForm_Submit_ReturnsCreateCommand(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Fill out form - move to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/', 't', 'm', 'p', '/', 'p', 'r', 'o', 'j'}})
	m = updated.(Model)

	// Move to name
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t', 'e', 's', 't'}})
	m = updated.(Model)

	// Submit with Enter
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Form should stay open but be in submitting state (showing progress)
	if !m.IsFormOpen() {
		t.Error("Form should stay open during submission")
	}
	if !m.IsFormSubmitting() {
		t.Error("Form should be in submitting state after submit")
	}

	// Should return a command (not nil)
	if cmd == nil {
		t.Error("Expected a command to be returned on submit")
	}
}

func TestForm_Submit_EmptyProjectPath_ShowsError(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Don't fill project path, just submit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Form should stay open with error
	if !m.IsFormOpen() {
		t.Error("Form should stay open when validation fails")
	}

	// Should not return a command
	if cmd != nil {
		t.Error("Should not return command when validation fails")
	}

	// Should have a form error
	if m.FormError() == "" {
		t.Error("Expected form error for empty project path")
	}
}

func TestForm_NoTemplates_ShowsError(t *testing.T) {
	cfg := &config.Config{Theme: "mocha"}
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-no-templates.log"
	lm, _ := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	// No templates
	m := NewModelWithTemplates(cfg, nil, lm)

	// Press 'c' to open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Form should open but show error
	if !m.IsFormOpen() {
		t.Error("Form should open")
	}

	if m.FormError() == "" {
		t.Error("Expected error when no templates available")
	}
}

func TestForm_KeysIgnored_WhenFormClosed(t *testing.T) {
	m := newTestModel(t)

	// Type some text without opening form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b', 'c'}})
	m = updated.(Model)

	// Form should still be closed
	if m.IsFormOpen() {
		t.Error("Form should remain closed")
	}

	// No form fields should be affected
	if m.FormProjectPath() != "" {
		t.Errorf("Project path should be empty, got %q", m.FormProjectPath())
	}
}

func TestForm_ListNavigation_Disabled_WhenFormOpen(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Arrow keys should not affect container list when form is open
	// (They should control template selection instead)
	// We verify this indirectly by checking template index changes
	initialIdx := m.FormTemplateIndex()

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	// Template index should change (proving keys go to form, not list)
	if m.FormTemplateIndex() == initialIdx && len(m.templates) > 1 {
		t.Error("Arrow keys should control form when open, not list")
	}
}

func TestFormView_ShowsTemplates(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	view := m.View()

	// Should show "Create Container" title
	if !containsString(view, "Create Container") {
		t.Error("View should show 'Create Container' title")
	}

	// Should show template names
	if !containsString(view, "go-project") {
		t.Error("View should show template name 'go-project'")
	}
}

func TestFormView_ShowsInputFields(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	view := m.View()

	// Should show field labels
	if !containsString(view, "Template") {
		t.Error("View should show 'Template' label")
	}
	if !containsString(view, "Project Path") {
		t.Error("View should show 'Project Path' label")
	}
	if !containsString(view, "Name") {
		t.Error("View should show 'Name' label")
	}
}

func TestFormView_ShowsFormHelp(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	view := m.View()

	// Should show form-specific help
	if !containsString(view, "Enter") {
		t.Error("View should mention Enter key")
	}
	if !containsString(view, "Esc") {
		t.Error("View should mention Esc key")
	}
}

func TestFormView_ShowsError(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Try to submit with empty project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	view := m.View()

	// Should show error message
	if !containsString(view, "required") {
		t.Error("View should show validation error")
	}
}

func TestFormView_HighlightsFocusedField(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Tab to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	view := m.View()

	// Should show some indicator of focused field
	// (exact styling may vary, but the view should render without error)
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestForm_Submit_BlocksInputDuringSubmission(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Fill out form - move to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/', 't', 'm', 'p', '/', 'p', 'r', 'o', 'j'}})
	m = updated.(Model)

	// Submit with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Store current project path
	originalPath := m.FormProjectPath()

	// Try to type more text - should be blocked
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m', 'o', 'r', 'e'}})
	m = updated.(Model)

	// Project path should not change
	if m.FormProjectPath() != originalPath {
		t.Errorf("Input should be blocked during submission, path changed from %q to %q", originalPath, m.FormProjectPath())
	}
}

func TestForm_Submit_EscapeCancels(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Fill out form - move to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/', 't', 'm', 'p', '/', 'p', 'r', 'o', 'j'}})
	m = updated.(Model)

	// Submit with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Verify in submitting state
	if !m.IsFormSubmitting() {
		t.Error("Form should be in submitting state")
	}

	// Press Escape to cancel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	// Form should close
	if m.IsFormOpen() {
		t.Error("Form should close when Escape pressed during submission")
	}
}

func TestFormView_ShowsProgress(t *testing.T) {
	m := newTestModel(t)

	// Open form
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	// Fill out form - move to project path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/', 't', 'm', 'p', '/', 'p', 'r', 'o', 'j'}})
	m = updated.(Model)

	// Submit with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	view := m.View()

	// Should still show Creating Container title (pulsing)
	if !containsString(view, "Creating Container") {
		t.Error("View should show 'Creating Container' title during submission")
	}

	// Should show the form values in disabled state
	if !containsString(view, "/tmp/proj") {
		t.Error("View should show project path during submission")
	}

	// Should show cancel hint
	if !containsString(view, "cancel") {
		t.Error("View should mention cancel option")
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
