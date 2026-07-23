package components

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// Message Components
// =============================================================================

// Styles for message display
var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Bold(true)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	toolUseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	toolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	codeBlockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	messageBoxStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
)

// ContentBlock represents a block of content in a message.
type ContentBlock struct {
	Type string `json:"type"`

	// For text blocks
	Text string `json:"text,omitempty"`

	// For tool_use blocks
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result blocks
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`

	// For thinking blocks
	Thinking string `json:"thinking,omitempty"`
}

// MessageModel represents a message for display.
type MessageModel struct {
	Role        string         `json:"role"`
	Content     []ContentBlock `json:"content"`
	Timestamp   string         `json:"timestamp,omitempty"`
	IsStreaming bool           `json:"isStreaming,omitempty"`
}

// RenderMessage renders a message with styling.
func RenderMessage(msg MessageModel, width int) string {
	var b strings.Builder

	// Render role prefix
	var prefix string
	switch msg.Role {
	case "user":
		prefix = userStyle.Render("You:")
	case "assistant":
		prefix = assistantStyle.Render("Claude:")
	case "system":
		prefix = systemStyle.Render("System:")
	default:
		prefix = msg.Role + ":"
	}

	b.WriteString(prefix + "\n")

	// Render content blocks
	for _, block := range msg.Content {
		renderedBlock := renderContentBlock(block, width-4)
		b.WriteString(messageBoxStyle.Render(renderedBlock) + "\n")
	}

	// Add timestamp if present
	if msg.Timestamp != "" {
		b.WriteString(timestampStyle.Render(msg.Timestamp) + "\n")
	}

	return b.String()
}

// renderContentBlock renders a single content block.
func renderContentBlock(block ContentBlock, width int) string {
	switch block.Type {
	case "text":
		return wrapText(block.Text, width)
	case "tool_use":
		return renderToolUse(block, width)
	case "tool_result":
		return renderToolResult(block, width)
	case "thinking":
		return renderThinking(block, width)
	default:
		return fmt.Sprintf("[%s block]", block.Type)
	}
}

// renderToolUse renders a tool use block.
func renderToolUse(block ContentBlock, width int) string {
	var b strings.Builder

	// Tool name with icon
	b.WriteString(toolUseStyle.Render("🔧 "+block.Name) + "\n")

	// Input preview (truncated if too long)
	if len(block.Input) > 0 {
		inputPreview := string(block.Input)
		if len(inputPreview) > 200 {
			inputPreview = inputPreview[:200] + "..."
		}
		b.WriteString(wrapText(inputPreview, width-2))
	}

	return b.String()
}

// renderToolResult renders a tool result block.
func renderToolResult(block ContentBlock, width int) string {
	var b strings.Builder

	// Result header
	icon := "✓"
	style := toolResultStyle
	if block.IsError {
		icon = "✗"
		style = errorStyle
	}

	b.WriteString(style.Render(icon+" Tool Result") + "\n")

	// Content preview
	contentStr := fmt.Sprintf("%v", block.Content)
	if len(contentStr) > 500 {
		contentStr = contentStr[:500] + "\n... (truncated)"
	}
	b.WriteString(wrapText(contentStr, width-2))

	return b.String()
}

// renderThinking renders a thinking block.
func renderThinking(block ContentBlock, width int) string {
	var b strings.Builder

	b.WriteString(systemStyle.Render("💭 Thinking:") + "\n")

	// Truncate thinking if too long
	thinking := block.Thinking
	if len(thinking) > 300 {
		thinking = thinking[:300] + "..."
	}
	b.WriteString(wrapText(thinking, width-2))

	return b.String()
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Wrap long lines
		for len(line) > width {
			// Find a good break point
			breakPoint := width
			for j := width - 1; j >= 0 && j >= width-20; j-- {
				if line[j] == ' ' || line[j] == '\t' {
					breakPoint = j
					break
				}
			}

			result.WriteString(line[:breakPoint] + "\n")
			line = line[breakPoint:]
			if line[0] == ' ' || line[0] == '\t' {
				line = line[1:]
			}
		}
		result.WriteString(line)
	}

	return result.String()
}

// =============================================================================
// Message List Component
// =============================================================================

// MessageListModel represents a list of messages.
type MessageListModel struct {
	Messages     []MessageModel
	ScrollOffset int
	MaxVisible   int
	Width        int
	Height       int
}

// NewMessageList creates a new message list.
func NewMessageList(width, height int) *MessageListModel {
	return &MessageListModel{
		Messages:     []MessageModel{},
		ScrollOffset: 0,
		MaxVisible:   height - 4, // Reserve space for input
		Width:        width,
		Height:       height,
	}
}

// AddMessage adds a message to the list.
func (m *MessageListModel) AddMessage(msg MessageModel) {
	m.Messages = append(m.Messages, msg)
	// Auto-scroll to bottom
	m.ScrollOffset = 0
}

// ScrollUp scrolls the rendered conversation up by one line.
func (m *MessageListModel) ScrollUp() {
	maxOffset := m.maxScrollOffset()
	if m.ScrollOffset < maxOffset {
		m.ScrollOffset++
	}
}

// ScrollDown scrolls the rendered conversation down by one line.
func (m *MessageListModel) ScrollDown() {
	if m.ScrollOffset > 0 {
		m.ScrollOffset--
	}
}

// PageUp scrolls upward by one viewport.
func (m *MessageListModel) PageUp() {
	step := m.MaxVisible - 1
	if step < 1 {
		step = 1
	}
	m.ScrollOffset += step
	maxOffset := m.maxScrollOffset()
	if m.ScrollOffset > maxOffset {
		m.ScrollOffset = maxOffset
	}
}

// PageDown scrolls downward by one viewport.
func (m *MessageListModel) PageDown() {
	step := m.MaxVisible - 1
	if step < 1 {
		step = 1
	}
	m.ScrollOffset -= step
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
}

// IsScrolled reports whether the viewport is above the latest messages.
func (m *MessageListModel) IsScrolled() bool {
	return m.ScrollOffset > 0
}

// View renders the message list as a fixed-height, line-based viewport.
func (m *MessageListModel) View() string {
	height := m.MaxVisible
	if height < 1 {
		height = 1
	}
	lines := m.renderedLines()
	maxOffset := len(lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.ScrollOffset > maxOffset {
		m.ScrollOffset = maxOffset
	}

	end := len(lines) - m.ScrollOffset
	start := end - height
	if start < 0 {
		start = 0
	}
	visible := append([]string(nil), lines[start:end]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n") + "\n"
}

func (m *MessageListModel) renderedLines() []string {
	if len(m.Messages) == 0 {
		return nil
	}
	var b strings.Builder
	for _, msg := range m.Messages {
		b.WriteString(RenderMessage(msg, m.Width))
		b.WriteString("\n")
	}
	rendered := strings.TrimSuffix(b.String(), "\n")
	return strings.Split(rendered, "\n")
}

func (m *MessageListModel) maxScrollOffset() int {
	maxOffset := len(m.renderedLines()) - m.MaxVisible
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}
