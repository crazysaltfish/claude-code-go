package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestInputModelUsesVisibleBlockCursor(t *testing.T) {
	if _, noColor := cursorStyle.GetBackground().(lipgloss.NoColor); noColor {
		t.Fatal("cursor must have a background color")
	}

	input := NewInput(">", "Type your message...", 80)
	view := input.View()
	if !containsAll(view, ">", "Type your message...") {
		t.Fatalf("empty input did not render cursor context and placeholder:\n%s", view)
	}
}

func TestInputModelAcceptsSpaceKey(t *testing.T) {
	input := NewInput(">", "", 80)
	input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	input.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})

	if input.Value != "hello world" {
		t.Fatalf("input value = %q, want %q", input.Value, "hello world")
	}
	if input.CursorPos != len(input.Value) {
		t.Fatalf("cursor position = %d, want %d", input.CursorPos, len(input.Value))
	}
}

func TestInputModelAcceptsConsecutiveSpaces(t *testing.T) {
	input := NewInput(">", "", 80)
	input.SetValue("hello")
	input.Update(tea.KeyMsg{Type: tea.KeySpace})
	input.Update(tea.KeyMsg{Type: tea.KeySpace})

	if input.Value != "hello  " {
		t.Fatalf("input value = %q, want two trailing spaces", input.Value)
	}
}

func TestInputModelInsertsSpaceAtCursor(t *testing.T) {
	input := NewInput(">", "", 80)
	input.SetValue("helloworld")
	input.CursorPos = len("hello")
	input.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})

	if input.Value != "hello world" {
		t.Fatalf("input value = %q, want %q", input.Value, "hello world")
	}
	if input.CursorPos != len("hello ") {
		t.Fatalf("cursor position = %d, want %d", input.CursorPos, len("hello "))
	}
}

func TestInputModelKeepsByteCursorValidForUnicodeRunes(t *testing.T) {
	input := NewInput(">", "", 80)
	input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("你好")})
	input.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})

	if input.Value != "你好 world" {
		t.Fatalf("input value = %q, want %q", input.Value, "你好 world")
	}
	if input.CursorPos != len(input.Value) {
		t.Fatalf("cursor position = %d, want %d", input.CursorPos, len(input.Value))
	}

	input.Update(tea.KeyMsg{Type: tea.KeyLeft})
	input.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if input.Value != "你好 word" {
		t.Fatalf("Unicode-aware editing produced %q", input.Value)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
