package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
