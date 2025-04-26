package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.design/x/clipboard"
)

type MyTextViewParams struct {
	Title string
	Text  string
}

type MyTextView struct {
	params   MyTextViewParams
	mainView *MainView

	flex               *tview.Flex
	tv                 *tview.TextView
	okBtn              *tview.Button
	copyToClipboardBtn *tview.Button
	frame              *tview.Frame
}

func NewMyTextView(
	mainView *MainView, params *MyTextViewParams,
) *MyTextView {
	rdv := &MyTextView{
		params:   *params,
		mainView: mainView,
	}

	var focusers []tview.Primitive
	getGenericTabHandler := func(curPrimitive tview.Primitive) func(event *tcell.EventKey) *tcell.EventKey {
		return func(event *tcell.EventKey) *tcell.EventKey {
			key := event.Key()

			nextIdx := 0
			prevIdx := 0

			for i, p := range focusers {
				if p != curPrimitive {
					continue
				}

				prevIdx = i - 1
				if prevIdx < 0 {
					prevIdx = len(focusers) - 1
				}

				nextIdx = i + 1
				if nextIdx >= len(focusers) {
					nextIdx = 0
				}
			}

			switch key {
			case tcell.KeyTab:
				rdv.mainView.params.App.SetFocus(focusers[nextIdx])
				return nil

			case tcell.KeyBacktab:
				rdv.mainView.params.App.SetFocus(focusers[prevIdx])
				return nil
			}

			return event
		}
	}

	rdv.flex = tview.NewFlex().SetDirection(tview.FlexRow)

	rdv.tv = tview.NewTextView()
	rdv.tv.SetText(params.Text)
	rdv.tv.SetDynamicColors(true)

	rdv.tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			rdv.Hide()
			return nil

		case tcell.KeyCtrlD:
			// TODO: ideally we'd want to only go half a page down, but for now just
			// return Ctrl+F which will go the full page down
			return tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			// TODO: ideally we'd want to only go half a page up, but for now just
			// return Ctrl+B which will go the full page up
			return tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)
		}

		event = rdv.genericInputHandler(event, getGenericTabHandler(rdv.tv), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	rdv.flex.AddItem(rdv.tv, 0, 1, true)
	focusers = append(focusers, rdv.tv)

	rdv.flex.AddItem(nil, 1, 0, false)

	rdv.okBtn = tview.NewButton("OK")
	rdv.okBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			rdv.Hide()
			return nil
		}

		event = rdv.genericInputHandler(event, getGenericTabHandler(rdv.okBtn), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	focusers = append(focusers, rdv.okBtn)

	rdv.copyToClipboardBtn = tview.NewButton("Copy to clipboard")
	rdv.copyToClipboardBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			// TODO: check if clipboard is actually available
			clipboard.Write(clipboard.FmtText, []byte(rdv.params.Text))
			rdv.copyToClipboardBtn.SetLabel("Copied!")
			return nil
		}

		rdv.copyToClipboardBtn.SetLabel("Copy to clipboard")

		event = rdv.genericInputHandler(event, getGenericTabHandler(rdv.copyToClipboardBtn), nil, nil)
		if event == nil {
			return nil
		}

		return event
	})
	focusers = append(focusers, rdv.copyToClipboardBtn)

	bottomFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	bottomFlex.
		AddItem(rdv.okBtn, 10, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(rdv.copyToClipboardBtn, 25, 0, false).
		AddItem(nil, 1, 0, false)

	rdv.flex.AddItem(bottomFlex, 1, 0, false)

	rdv.frame = tview.NewFrame(rdv.flex).SetBorders(0, 0, 0, 0, 0, 0)
	rdv.frame.SetBorder(true).SetBorderPadding(0, 0, 1, 1)
	rdv.frame.SetTitle(rdv.params.Title)

	return rdv
}

func (rdv *MyTextView) Show() {
	rdv.mainView.showModal(
		pageNameTextView, rdv.frame,
		121,
		35,
		true,
	)
}

func (rdv *MyTextView) Hide() {
	rdv.mainView.hideModal(pageNameTextView, true)
}

func (rdv *MyTextView) genericInputHandler(
	event *tcell.EventKey,
	genericTabHandler func(event *tcell.EventKey) *tcell.EventKey,
	getQFPart func(qf QueryFull) string,
	setQFPart func(qf *QueryFull, part string),
) *tcell.EventKey {
	event = genericTabHandler(event)
	if event == nil {
		return nil
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		rdv.Hide()
		return nil

	case tcell.KeyRune:
		if event.Rune() == 'q' {
			rdv.Hide()
			return nil
		}
	}

	return event
}
