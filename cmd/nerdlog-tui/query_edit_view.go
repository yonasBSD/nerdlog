package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var queryLabelText = `Query (awk syntax). Examples:
- Simple regexp:                          [yellow]/foo bar/[-]
- Regexps with complex conditions:        [yellow]( /foo bar/ || /other stuff/ ) && !/baz/[-]
- Find items tagged with series ID 86:    [yellow]/series_ids_string=.*\|86\|.*/[-]
- Find errors:                            [yellow]/level_name=error/[-]
`

/*
var timeLabelText = `Time range. Both "From" and "To" can either be absolute like "[yellow]Mar27 12:00[-]", or relative
like "[yellow]-2h30m[-]" (relative to current time). The "To" can also be "now" or just an empty string,
in which case the current time will be used. Absolute time is in UTC.
`
*/

var timeLabelText = `Time range in the format "[yellow]<time>[ to <time>][-]", where [yellow]<time>[-] is either absolute like "[yellow]Mar27 12:00[-]"
(in UTC), or relative like "[yellow]-2h30m[-]" (relative to current time). If the "to" part is omitted,
current time is used. Examples:
[yellow]1h[-]

like "[yellow]-2h30m[-]" (relative to current time). The "To" can also be "now" or just an empty string,
in which case the current time will be used. Absolute time is in UTC.
`

var hostsLabelText = `Hosts. TODO explain
`

type QueryEditViewParams struct {
	// DoneFunc is called when the user submits the form. If it returns a non-nil
	// error, the form will show that error and will not be submitted.
	DoneFunc func(data QueryEditData) error

	// TODO: callback for editing nodes, to show in realtime how many nodes matched
}

type QueryEditData struct {
	Time        string
	HostsFilter string
	Query       string
}

type QueryEditView struct {
	params   QueryEditViewParams
	mainView *MainView

	data QueryEditData

	flex *tview.Flex

	timeInput  *tview.InputField
	hostsInput *tview.InputField
	queryInput *tview.InputField

	frame *tview.Frame
	//
	//textView *tview.TextView
	//buttons  []*tview.Button
}

func NewQueryEditView(
	mainView *MainView, params *QueryEditViewParams,
) *QueryEditView {
	qev := &QueryEditView{
		params:   *params,
		mainView: mainView,
	}

	//if qev.params.Width == 0 {
	//qev.params.Width = 40
	//}

	//if qev.params.Height == 0 {
	//qev.params.Height = 10
	//}

	var focusers []tview.Primitive
	getDoneFunc := func(curPrimitive tview.Primitive) func(key tcell.Key) {
		return func(key tcell.Key) {
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
				qev.mainView.params.App.SetFocus(focusers[nextIdx])

			case tcell.KeyBacktab:
				qev.mainView.params.App.SetFocus(focusers[prevIdx])

			case tcell.KeyEsc:
				qev.Hide()
			}
		}
	}

	qev.flex = tview.NewFlex().SetDirection(tview.FlexRow)

	timeLabel := tview.NewTextView()
	timeLabel.SetText(timeLabelText)
	timeLabel.SetDynamicColors(true)
	qev.flex.AddItem(timeLabel, 3, 0, false)

	qev.timeInput = tview.NewInputField()
	qev.timeInput.SetDoneFunc(getDoneFunc(qev.timeInput))
	qev.flex.AddItem(qev.timeInput, 1, 0, true)
	focusers = append(focusers, qev.timeInput)

	qev.flex.AddItem(nil, 1, 0, false)

	hostsLabel := tview.NewTextView()
	hostsLabel.SetText(hostsLabelText)
	qev.flex.AddItem(hostsLabel, 1, 0, false)

	qev.hostsInput = tview.NewInputField()
	qev.hostsInput.SetDoneFunc(getDoneFunc(qev.hostsInput))
	qev.flex.AddItem(qev.hostsInput, 1, 0, false)
	focusers = append(focusers, qev.hostsInput)

	qev.flex.AddItem(nil, 1, 0, false)

	queryLabel := tview.NewTextView()
	queryLabel.SetText(queryLabelText)
	queryLabel.SetDynamicColors(true)
	qev.flex.AddItem(queryLabel, 5, 0, false)

	qev.queryInput = tview.NewInputField()
	qev.queryInput.SetDoneFunc(getDoneFunc(qev.queryInput))
	qev.flex.AddItem(qev.queryInput, 1, 0, false)
	focusers = append(focusers, qev.queryInput)

	//qev.textView = tview.NewTextView()
	//qev.textView.SetText(params.Message)
	//qev.textView.SetTextAlign(tview.AlignCenter)

	//qev.flex.AddItem(qev.textView, 0, 1, len(params.Buttons) == 0)

	//for i, b := range params.Buttons {
	//btnLabel := b
	//btnIdx := i
	//btn := tview.NewButton(btnLabel).SetSelectedFunc(func() {
	//params.OnButtonPressed(btnLabel, btnIdx)
	//})

	//// TODO: use horizontal flex for buttons
	//qev.flex.AddItem(btn, 1, 0, i == 0)
	//}

	qev.frame = tview.NewFrame(qev.flex).SetBorders(0, 0, 0, 0, 0, 0)
	qev.frame.SetBorder(true).SetBorderPadding(1, 1, 1, 1)
	qev.frame.SetTitle("Edit query params")

	return qev
}

func (qev *QueryEditView) Show(data QueryEditData) {
	qev.data = data
	qev.timeInput.SetText(qev.data.Time)
	qev.hostsInput.SetText(qev.data.HostsFilter)
	qev.queryInput.SetText(qev.data.Query)
	qev.mainView.showModal(
		pageNameEditQueryParams, qev.frame,
		101,
		20,
	)
}

func (qev *QueryEditView) Hide() {
	qev.mainView.hideModal(pageNameEditQueryParams)
}
