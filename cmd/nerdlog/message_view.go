package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type MessageViewParams struct {
	App *tview.Application

	MessageID       string
	Title           string
	Message         string
	Buttons         []string
	OnButtonPressed func(label string, idx int)
	OnEsc           func()

	// Width and Height are 40 and 10 by default
	Width, Height int

	// By default, tview.AlignLeft (because it happens to be 0)
	Align int

	NoFocus bool

	BackgroundColor tcell.Color
}

type MessageView struct {
	params   MessageViewParams
	mainView *MainView

	msgboxFlex  *tview.Flex
	buttonsFlex *tview.Flex
	frame       *tview.Frame

	textView *tview.TextView
	focusers []tview.Primitive
}

// getMaxLineLength returns the length of the longest line in the given string.
func getMaxLineLength(s string) int {
	maxLen := 0
	start := 0

	for i, c := range s {
		if c == '\n' {
			lineLen := i - start
			if lineLen > maxLen {
				maxLen = lineLen
			}
			start = i + 1
		}
	}

	// Handle the last line if it doesn't end with a newline
	if len(s)-start > maxLen {
		maxLen = len(s) - start
	}

	return maxLen
}

// getNumLines returns the number of lines that are needed to draw the given
// text.
func getNumLines(s string, screenWidth int) int {
	if screenWidth <= 0 {
		return 0
	}

	lines := strings.Split(s, "\n")
	numLines := 0
	for _, line := range lines {
		// Divide line length by screen width and round up
		lineLen := len(line)
		numLines += (lineLen + screenWidth - 1) / screenWidth
	}
	return numLines
}

func NewMessageView(
	mainView *MainView, params *MessageViewParams,
) *MessageView {
	msgv := &MessageView{
		params:   *params,
		mainView: mainView,
	}

	// extraWidth covers padding and border
	extraWidth := 4
	// extraHeight covers padding, border and buttons
	extraHeight := 6

	if msgv.params.Width == 0 {
		// Set it to fit the longest line (if the terminal window allows),
		// plus the padding and the border.
		msgv.params.Width = getMaxLineLength(params.Message) + extraWidth
	}

	if msgv.params.Height == 0 {
		msgv.params.Height = extraHeight + getNumLines(params.Message, mainView.screenWidth-extraWidth)
	}

	msgv.msgboxFlex = tview.NewFlex().SetDirection(tview.FlexRow)

	msgv.textView = tview.NewTextView()
	msgv.textView.SetText(params.Message)
	msgv.textView.SetTextAlign(msgv.params.Align)
	msgv.textView.SetDynamicColors(true)

	if msgv.params.BackgroundColor != tcell.ColorDefault {
		msgv.textView.SetBackgroundColor(msgv.params.BackgroundColor)
	}

	msgv.msgboxFlex.AddItem(msgv.textView, 0, 1, len(params.Buttons) == 0)

	msgv.buttonsFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	msgv.msgboxFlex.AddItem(msgv.buttonsFlex, 1, 1, len(params.Buttons) != 0)

	// Add a spacer at the left of the buttons, to make them centered
	// (there's also a spacer at the right, added later)
	msgv.buttonsFlex.AddItem(nil, 0, 1, false)

	for i := 0; i < len(params.Buttons); i++ {
		btnLabel := params.Buttons[i]
		btnIdx := i
		btn := tview.NewButton(btnLabel).SetSelectedFunc(func() {
			params.OnButtonPressed(btnLabel, btnIdx)
		})
		msgv.focusers = append(msgv.focusers, btn)
		tabHandler := msgv.getGenericTabHandler(btn)
		btn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			// Handle Esc key
			switch event.Key() {
			case tcell.KeyEsc:
				if params.OnEsc != nil {
					params.OnEsc()
				}
			}

			event = tabHandler(event)
			if event == nil {
				return nil
			}

			return event
		})

		// Unless it's the first button, add a 1-char spacing.
		if i > 0 {
			msgv.buttonsFlex.AddItem(nil, 1, 0, false)
		}

		// Add the button itself: spacing of 2 chars at each side, and min 10 chars total.
		// Focus the first one.
		buttonSize := len(btnLabel) + 2*2
		if buttonSize < 10 {
			buttonSize = 10
		}
		msgv.buttonsFlex.AddItem(btn, buttonSize, 0, i == 0)
	}

	// Add a spacer at the right of the buttons, to make them centered
	// (there's also a spacer at the left, added before)
	msgv.buttonsFlex.AddItem(nil, 0, 1, false)

	msgv.frame = tview.NewFrame(msgv.msgboxFlex).SetBorders(0, 0, 0, 0, 0, 0)
	msgv.frame.SetBorder(true).SetBorderPadding(1, 1, 1, 1)
	msgv.frame.SetTitle(params.Title)
	if msgv.params.BackgroundColor != tcell.ColorDefault {
		msgv.frame.SetBackgroundColor(msgv.params.BackgroundColor)
	}

	return msgv
}

func (msgv *MessageView) Show() {
	msgv.mainView.showModal(
		pageNameMessage+msgv.params.MessageID, msgv.frame,
		msgv.params.Width,
		msgv.params.Height,
		!msgv.params.NoFocus,
	)
}

func (msgv *MessageView) Hide() {
	msgv.mainView.hideModal(pageNameMessage+msgv.params.MessageID, !msgv.params.NoFocus)
}

func (msgv *MessageView) getGenericTabHandler(curPrimitive tview.Primitive) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()

		nextIdx := 0
		prevIdx := 0

		for i, p := range msgv.focusers {
			if p != curPrimitive {
				continue
			}

			prevIdx = i - 1
			if prevIdx < 0 {
				prevIdx = len(msgv.focusers) - 1
			}

			nextIdx = i + 1
			if nextIdx >= len(msgv.focusers) {
				nextIdx = 0
			}
		}

		switch key {
		case tcell.KeyTab:
			msgv.params.App.SetFocus(msgv.focusers[nextIdx])
			return nil

		case tcell.KeyBacktab:
			msgv.params.App.SetFocus(msgv.focusers[prevIdx])
			return nil
		}

		return event
	}
}
