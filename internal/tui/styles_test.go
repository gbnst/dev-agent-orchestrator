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
