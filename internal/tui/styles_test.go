package tui

import (
	"testing"
)

func TestStyles_TabStyles(t *testing.T) {
	styles := NewStyles("mocha")

	// Verify tab styles exist and return non-empty styles
	activeStyle := styles.ActiveTabStyle()
	if activeStyle.GetBold() != true {
		t.Error("ActiveTabStyle should be bold")
	}

	inactiveStyle := styles.InactiveTabStyle()
	// Inactive should not be bold
	if inactiveStyle.GetBold() == true {
		t.Error("InactiveTabStyle should not be bold")
	}

	// Tab gap should return a string
	gap := styles.TabGapFill()
	if gap == "" {
		t.Error("TabGapFill should return a non-empty string")
	}
}

func TestStyles_AllFlavors(t *testing.T) {
	flavors := []string{"latte", "frappe", "macchiato", "mocha"}

	for _, flavor := range flavors {
		t.Run(flavor, func(t *testing.T) {
			styles := NewStyles(flavor)

			// Verify all tab styles work for each flavor
			_ = styles.ActiveTabStyle()
			_ = styles.InactiveTabStyle()
			_ = styles.TabGapFill()

			// Existing styles should still work
			_ = styles.TitleStyle()
			_ = styles.ErrorStyle()
		})
	}
}

func TestStyles_StatusStyles(t *testing.T) {
	styles := NewStyles("mocha")

	// SuccessStyle should exist and be renderable
	successStyle := styles.SuccessStyle()
	rendered := successStyle.Render("test")
	if rendered == "" {
		t.Error("SuccessStyle should render content")
	}

	// InfoStatusStyle should exist
	infoStyle := styles.InfoStatusStyle()
	rendered = infoStyle.Render("test")
	if rendered == "" {
		t.Error("InfoStatusStyle should render content")
	}

	// ErrorStyle already exists, just verify it works
	errorStyle := styles.ErrorStyle()
	if !errorStyle.GetBold() {
		t.Error("ErrorStyle should be bold")
	}
}

func TestStyles_TreeItemSelectedStyle(t *testing.T) {
	styles := NewStyles("mocha")

	selectedStyle := styles.TreeItemSelectedStyle()
	rendered := selectedStyle.Render("test")
	if rendered == "" {
		t.Error("TreeItemSelectedStyle should render content")
	}

	if !selectedStyle.GetBold() {
		t.Error("TreeItemSelectedStyle should be bold")
	}
}

func TestStyles_PanelHeaderStyles(t *testing.T) {
	styles := NewStyles("mocha")

	focusedStyle := styles.PanelHeaderFocusedStyle()
	if !focusedStyle.GetBold() {
		t.Error("PanelHeaderFocusedStyle should be bold")
	}
	if focusedStyle.Render("Test") == "" {
		t.Error("PanelHeaderFocusedStyle should render content")
	}

	unfocusedStyle := styles.PanelHeaderUnfocusedStyle()
	if unfocusedStyle.GetBold() {
		t.Error("PanelHeaderUnfocusedStyle should not be bold")
	}
	if unfocusedStyle.Render("Test") == "" {
		t.Error("PanelHeaderUnfocusedStyle should render content")
	}
}

func TestStyles_LogLevelBadges(t *testing.T) {
	styles := NewStyles("mocha")

	// All log styles should exist and be callable
	_ = styles.LogDebugStyle()
	_ = styles.LogInfoStyle()
	_ = styles.LogWarnStyle()
	_ = styles.LogErrorStyle()

	// ERROR style should be bold
	errorStyle := styles.LogErrorStyle()
	if !errorStyle.GetBold() {
		t.Error("LogErrorStyle should be bold")
	}

	// All styles should be renderable (not nil)
	debugStyle := styles.LogDebugStyle()
	rendered := debugStyle.Render("DEBUG")
	if rendered == "" {
		t.Error("LogDebugStyle should render content")
	}
}
