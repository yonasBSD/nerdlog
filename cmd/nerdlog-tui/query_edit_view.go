package main

import (
	"github.com/dimonomid/nerdlog/clhistory"
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
var timeLabelText = `Time range. Both "From" and "To" can either be absolute like "[yellow]Mar27_12:00[-]", or relative
like "[yellow]-2h30m[-]" (relative to current time). The "To" can also be "now" or just an empty string,
in which case the current time will be used. Absolute time is in UTC.
`
*/

var timeLabelText = `Time range in the format "[yellow]<time>[ to <time>][-]", where [yellow]<time>[-] is either absolute like "[yellow]Mar27 12:00[-]"
(in UTC), or relative like "[yellow]-2h30m[-]" (relative to current time). If the "to" part is omitted,
current time is used.
`

var hostsLabelText = `Hosts. Comma-separated glob patterns, e.g. "[yellow]my-host-*,my-host-*[-]" matches
all staging redacted and redacted nodes.`

type QueryEditViewParams struct {
	// DoneFunc is called when the user submits the form. If it returns a non-nil
	// error, the form will show that error and will not be submitted.
	DoneFunc func(data QueryFull, dqp doQueryParams) error

	// TODO: callback for editing nodes, to show in realtime how many nodes matched

	History *clhistory.CLHistory
}

type QueryEditView struct {
	params   QueryEditViewParams
	mainView *MainView

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

			case tcell.KeyEnter:
				err := qev.params.DoneFunc(qev.GetQueryFull(), doQueryParams{})
				if err != nil {
					qev.mainView.showMessagebox("err", "Error", err.Error(), nil)
					break
				}

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
	hostsLabel.SetDynamicColors(true)
	qev.flex.AddItem(hostsLabel, 2, 0, false)

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

	qev.timeInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = qev.genericHistoryInputHandler(
			event,
			func(qf QueryFull) string { return qf.Time },
			func(qf *QueryFull, part string) { qf.Time = part },
		)
		if event == nil {
			return nil
		}

		return event
	})

	qev.hostsInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = qev.genericHistoryInputHandler(
			event,
			func(qf QueryFull) string { return qf.HostsFilter },
			func(qf *QueryFull, part string) { qf.HostsFilter = part },
		)
		if event == nil {
			return nil
		}

		return event
	})

	qev.queryInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = qev.genericHistoryInputHandler(
			event,
			func(qf QueryFull) string { return qf.Query },
			func(qf *QueryFull, part string) { qf.Query = part },
		)
		if event == nil {
			return nil
		}

		return event
	})

	return qev
}

func (qev *QueryEditView) Show(data QueryFull) {
	qev.SetQueryFull(data)
	qev.mainView.showModal(
		pageNameEditQueryParams, qev.frame,
		101,
		20,
		true,
	)
}

func (qev *QueryEditView) Hide() {
	qev.mainView.hideModal(pageNameEditQueryParams, true)
}

func (qev *QueryEditView) GetQueryFull() QueryFull {
	return QueryFull{
		Time:        qev.timeInput.GetText(),
		Query:       qev.queryInput.GetText(),
		HostsFilter: qev.hostsInput.GetText(),
	}
}

func (qev *QueryEditView) SetQueryFull(qf QueryFull) {
	qev.timeInput.SetText(qf.Time)
	qev.hostsInput.SetText(qf.HostsFilter)
	qev.queryInput.SetText(qf.Query)
}

func (qev *QueryEditView) genericHistoryInputHandler(
	event *tcell.EventKey,
	getQFPart func(qf QueryFull) string,
	setQFPart func(qf *QueryFull, part string),
) *tcell.EventKey {
	qf := qev.GetQueryFull()
	cmd := qf.MarshalShellCmd()

	qfPart := getQFPart(qf)

	var itemToUse *clhistory.Item

	switch event.Key() {

	// On Ctrl+K, Ctrl+J list history over all fields.

	case tcell.KeyCtrlK:
		item, _ := qev.params.History.Prev(cmd)
		itemToUse = &item

	case tcell.KeyCtrlJ:
		item, _ := qev.params.History.Next(cmd)
		itemToUse = &item

	// On Ctrl+P, Ctrl+N list history over only the current field.  This is kind
	// of a hack since we're still using the common history, and manually
	// skipping the items with the same values for this particular field.  Maybe
	// it'd be easier to just keep separate history files for every field, idk.

	case tcell.KeyCtrlP, tcell.KeyUp, tcell.KeyCtrlN, tcell.KeyDown:
		var item clhistory.Item

		for {
			var hasMore bool
			if event.Key() == tcell.KeyCtrlP || event.Key() == tcell.KeyUp {
				item, hasMore = qev.params.History.Prev(cmd)
			} else {
				item, hasMore = qev.params.History.Next(cmd)
			}

			var tmp QueryFull
			if err := tmp.UnmarshalShellCmd(item.Str); err != nil {
				qev.mainView.showMessagebox("err", "Broken query history", err.Error(), nil)
				return nil
			}

			curQFPart := getQFPart(tmp)
			if (curQFPart != "" && curQFPart != qfPart) || !hasMore {
				// Either we found a different value for this field, or ran out of
				// history. Set this value in the original QueryFull, and use it.
				setQFPart(&qf, curQFPart)
				item.Str = qf.MarshalShellCmd()
				break
			}
		}

		itemToUse = &item
	}

	if itemToUse != nil {
		if err := qf.UnmarshalShellCmd(itemToUse.Str); err != nil {
			qev.mainView.showMessagebox("err", "Broken query history", err.Error(), nil)
			return nil
		}

		qev.SetQueryFull(qf)

		return nil
	}

	// If the field was edited by the user, reset the history current position.
	switch event.Key() {
	case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2,
		tcell.KeyDelete, tcell.KeyCtrlD,
		tcell.KeyCtrlW, tcell.KeyCtrlU, tcell.KeyCtrlK:

		qev.params.History.Reset()
	}

	return event
}
