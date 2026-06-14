// Package ui provides tview widget factory functions.
// It mirrors the role of widgets.ts: create and style widgets once; the view
// layer wires them together into a layout and handles all interactions.
package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	clr "github.com/imaustink/hatui/internal/color"
)

// ─────────────────────────────────────────────── Widget factories ──

// NewScreen creates and configures the root tview.Application.
func NewApp() *tview.Application {
	return tview.NewApplication().
		EnableMouse(false)
}

// NewHeader creates the 4-row header TextView.
func NewHeader() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	tv.SetBorder(false)
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGHeader)))
	return tv
}

// NewEntityTable creates the scrollable entity list Table (left panel, 70%).
func NewEntityTable() *tview.Table {
	t := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0) // freeze header row
	t.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BG)))
	t.SetSelectedStyle(tcell.Style{}.
		Background(tcell.NewHexColor(hexToInt(clr.COLORS.BGSelected))).
		Foreground(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan))).
		Attributes(tcell.AttrBold))
	return t
}

// NewDetailPanel creates the detail TextView (right panel, 30%).
func NewDetailPanel() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	tv.SetBorder(true)
	tv.SetBorderColor(tcell.NewHexColor(hexToInt(clr.COLORS.Border)))
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGPanel)))
	tv.SetTitle(" Detail ")
	tv.SetTitleColor(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan)))
	return tv
}

// NewStatusBar creates the one-line status bar TextView.
func NewStatusBar() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	tv.SetBorder(false)
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGAlt)))
	return tv
}

// NewCommandBar creates the command/filter input bar.
func NewCommandBar() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	tv.SetBorder(false)
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BG)))
	return tv
}

// NewHelpOverlay creates the help overlay Modal (hidden by default).
func NewHelpOverlay() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true)
	tv.SetBorderColor(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan)))
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGAlt)))
	tv.SetTitle(" Help — ? to close ")
	tv.SetTitleColor(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan)))
	return tv
}

// NewToast creates the toast notification box (hidden by default).
func NewToast() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	tv.SetBorder(true)
	tv.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGHighlight)))
	return tv
}

// NewAutocompleteList creates the autocomplete suggestion list.
func NewAutocompleteList() *tview.List {
	l := tview.NewList().
		ShowSecondaryText(false)
	l.SetBorder(true)
	l.SetBorderColor(tcell.NewHexColor(hexToInt(clr.COLORS.BorderActive)))
	l.SetBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGAlt)))
	l.SetMainTextColor(tcell.NewHexColor(hexToInt(clr.COLORS.TextPrimary)))
	l.SetSelectedTextColor(tcell.NewHexColor(hexToInt(clr.COLORS.Cyan)))
	l.SetSelectedBackgroundColor(tcell.NewHexColor(hexToInt(clr.COLORS.BGSelected)))
	return l
}

// ─────────────────────────────────────────────── Layout helpers ──

// BuildMainLayout constructs the primary tview.Flex layout.
//
//	vertical:
//	  header       (height 4)
//	  inner flex   (flex 1)
//	    table      (flex 7)
//	    detail     (flex 3)
//	  status bar   (height 1)
//	  command bar  (height 1)
func BuildMainLayout(header, detail, statusBar, cmdBar tview.Primitive, table *tview.Table) *tview.Flex {
	inner := tview.NewFlex().
		AddItem(table, 0, 7, true).
		AddItem(detail, 0, 3, false)

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 4, 0, false).
		AddItem(inner, 0, 1, true).
		AddItem(statusBar, 1, 0, false).
		AddItem(cmdBar, 2, 0, false)
}

// ─────────────────────────────────────────────── Color helpers ──

// hexToInt converts a "#rrggbb" string to a int32 for tcell.NewHexColor.
func hexToInt(hex string) int32 {
	hex = trimHash(hex)
	if len(hex) != 6 {
		return 0
	}
	var r, g, b int32
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return (r << 16) | (g << 8) | b
}

func trimHash(s string) string {
	if len(s) > 0 && s[0] == '#' {
		return s[1:]
	}
	return s
}
