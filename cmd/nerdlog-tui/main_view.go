package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dimonomid/nerdlog/core"
	"github.com/gdamore/tcell/v2"
	"github.com/juju/errors"
	"github.com/rivo/tview"
)

const logsTableTimeLayout = "Jan02 15:04:05.000"

const (
	pageNameMessage         = "message"
	pageNameEditQueryParams = "message"
)

type MainViewParams struct {
	InitialHostsFilter string

	App *tview.Application

	// OnLogQuery is called by MainView whenever the user submits a query to get
	// logs.
	OnLogQuery OnLogQueryCallback

	OnHostsFilterChange OnHostsFilterChange

	// TODO: support command history
	OnCmd OnCmdCallback
}

type MainView struct {
	params    MainViewParams
	rootPages *tview.Pages
	logsTable *tview.Table

	queryInput *tview.InputField
	cmdInput   *tview.InputField

	topFlex      *tview.Flex
	queryEditBtn *tview.Button
	timeLabel    *tview.TextView

	queryEditView *QueryEditView

	// focusedBeforeCmd is a primitive which was focused before cmdInput was
	// focused. Once the user is done editing command, focusedBeforeCmd
	// normally resumes focus.
	focusedBeforeCmd tview.Primitive

	histogram *Histogram

	statusLine *tview.TextView

	hostsFilter string

	// from, to represent the selected time range
	from, to TimeOrDur

	// query is the effective search query
	query string

	// actualFrom, actualTo represent the actual time range resolved from from
	// and to, and they both can't be zero.
	actualFrom, actualTo time.Time

	curLogResp *core.LogResp
	// statsFrom and statsTo represent the first and last element present
	// in curLogResp.MinuteStats. Note that this range might be smaller than
	// (from, to), because for some minute stats might be missing. statsFrom
	// and statsTo are only useful for cases when from and/or to are zero (meaning,
	// time range isn't limited)
	statsFrom, statsTo time.Time

	//marketViewsByID map[common.MarketID]*MarketView
	//marketDescrByID map[common.MarketID]MarketDescr

	modalsFocusStack []tview.Primitive
}

type OnLogQueryCallback func(params core.QueryLogsParams)
type OnHostsFilterChange func(hostsFilter string) error
type OnCmdCallback func(cmd string)

func NewMainView(params *MainViewParams) *MainView {
	mv := &MainView{
		params: *params,
	}

	mv.setHostsFilter(params.InitialHostsFilter)

	mv.rootPages = tview.NewPages()

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	queryInputStaleMatch := tcell.Style{}.
		Background(tcell.ColorBlue).
		Foreground(tcell.ColorWhite).
		Bold(true)

	queryInputStaleMismatch := tcell.Style{}.
		Background(tcell.ColorDarkRed).
		Foreground(tcell.ColorWhite).
		Bold(true)

	queryInputApplyStyle := func() {
		style := queryInputStaleMatch
		if mv.queryInput.GetText() != mv.query {
			style = queryInputStaleMismatch
		}

		mv.queryInput.SetFieldStyle(style)
	}

	mv.queryInput = tview.NewInputField()
	mv.queryInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			mv.query = mv.queryInput.GetText()
			mv.doQuery()
			queryInputApplyStyle()
		case tcell.KeyEsc:
			if mv.queryInput.GetText() != mv.query {
				mv.queryInput.SetText(mv.query)
				queryInputApplyStyle()
			}
		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.queryEditBtn)
		case tcell.KeyBacktab:
			mv.params.App.SetFocus(mv.logsTable)
		}
	})

	mv.queryInput.SetChangedFunc(func(text string) {
		queryInputApplyStyle()
	})

	queryInputApplyStyle()

	mv.queryEditBtn = tview.NewButton("Edit")
	mv.queryEditBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.logsTable)
		case tcell.KeyBacktab:
			mv.params.App.SetFocus(mv.queryInput)
			return nil
		}

		return event
	})
	mv.queryEditBtn.SetSelectedFunc(func() {
		ftr := FromToRange{mv.from, mv.to}
		mv.queryEditView.Show(QueryEditData{
			Time:        ftr.String(),
			Query:       mv.query,
			HostsFilter: mv.hostsFilter,
		})
	})

	queryLabel := tview.NewTextView()
	queryLabel.SetScrollable(false).SetText("Query:")

	mv.timeLabel = tview.NewTextView()
	mv.timeLabel.SetScrollable(false)

	mv.topFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	mv.topFlex.
		AddItem(queryLabel, 6, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.queryInput, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(mv.timeLabel, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.queryEditBtn, 6, 0, false)

	mainFlex.AddItem(mv.topFlex, 1, 0, true)

	mv.histogram = NewHistogram()
	mv.histogram.SetBinSize(60) // 1 minute
	mv.histogram.SetXFormatter(func(v int) string {
		t := time.Unix(int64(v), 0).UTC()
		return t.Format("15:04")
	})
	mv.histogram.SetXMarker(func(from, to int, numChars int) []int {
		// TODO proper impl
		step := (to - from) / 6
		if step == 0 {
			return nil
		}

		ret := []int{}
		for i := from; i <= to; i += step {
			ret = append(ret, i)
		}

		return ret
	})
	mainFlex.AddItem(mv.histogram, 6, 0, false)

	mv.logsTable = tview.NewTable()
	mv.updateTableHeader(nil)

	//mv.logsTable.SetEvaluateAllRows(true)
	mv.logsTable.SetFocusFunc(func() {
		mv.logsTable.SetSelectable(true, false)
	})
	mv.logsTable.SetBlurFunc(func() {
		mv.logsTable.SetSelectable(false, false)
	})

	mv.logsTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()

		switch key {
		case tcell.KeyCtrlD:
			// TODO: ideally we'd want to only go half a page down, but for now just
			// return Ctrl+F which will go the full page down
			return tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			// TODO: ideally we'd want to only go half a page up, but for now just
			// return Ctrl+B which will go the full page up
			return tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)

		case tcell.KeyRune:
			switch event.Rune() {
			case ':':
				mv.cmdInput.SetText(":")
				mv.focusedBeforeCmd = mv.params.App.GetFocus()
				mv.params.App.SetFocus(mv.cmdInput)
			}
		}

		return event
	})

	// TODO: once tableview fixed, use SetFixed(1, 1)
	// (there's an issue with going to the very top using "g")
	mv.logsTable.SetFixed(1, 1)
	mv.logsTable.Select(0, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			//mv.logsTable.SetSelectable(true, true)
		}
		if key == tcell.KeyTab {
			mv.params.App.SetFocus(mv.queryInput)
		}
		if key == tcell.KeyBacktab {
			mv.params.App.SetFocus(mv.queryEditBtn)
		}
	}).SetSelectedFunc(func(row int, column int) {
		// TODO: instead of showing current cell contents, show original raw message
		mv.showMessagebox("msg", "Message", mv.logsTable.GetCell(row, 1).Text, nil)
	}) // TODO .SetInputCapture

	/*

		lorem := strings.Split("Lorem iipsum-[:red:b]ipsum[:-:-]-ipsum-[::b]ipsum[::-]-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-ipsum-psum- dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet. Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet.", " ")
		cols, rows := 10, 400
		word := 0
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				color := tcell.ColorWhite
				if c < 1 || r < 1 {
					color = tcell.ColorYellow
				}
				mv.logsTable.SetCell(r, c,
					tview.NewTableCell(lorem[word]).
						SetTextColor(color).
						SetAlign(tview.AlignLeft))
				word = (word + 1) % len(lorem)
			}
		}
	*/

	mainFlex.AddItem(mv.logsTable, 0, 1, false)

	mv.statusLine = tview.NewTextView()
	mv.statusLine.SetScrollable(false).SetDynamicColors(true)

	mainFlex.AddItem(mv.statusLine, 1, 0, false)

	mv.cmdInput = tview.NewInputField()
	mv.cmdInput.SetChangedFunc(func(text string) {
		if text == "" {
			mv.params.App.SetFocus(mv.focusedBeforeCmd)
		}
	})

	mv.cmdInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			cmd := mv.cmdInput.GetText()

			// Remove the ":" prefix
			cmd = cmd[1:]

			if cmd != "" {
				mv.params.OnCmd(cmd)
			}

		case tcell.KeyEsc:
		// Gonna just stop editing it
		default:
			// Ignore it
			return
		}

		mv.cmdInput.SetText("")
	})

	mainFlex.AddItem(mv.cmdInput, 1, 0, false)

	mv.queryEditView = NewQueryEditView(mv, &QueryEditViewParams{
		DoneFunc: func(data QueryEditData) error {
			err := mv.params.OnHostsFilterChange(data.HostsFilter)
			if err != nil {
				return errors.Trace(err)
			}

			return nil
		},
	})

	mv.rootPages.AddPage("mainFlex", mainFlex, true, true)

	return mv
}

func (mv *MainView) GetUIPrimitive() tview.Primitive {
	return mv.rootPages
}

func (mv *MainView) ApplyHMState(hmState *core.HostsManagerState) {
	mv.params.App.QueueUpdateDraw(func() {
		sb := strings.Builder{}

		if !hmState.Connected {
			sb.WriteString("connecting ")
		} else if hmState.Busy {
			sb.WriteString("busy ")
		} else {
			sb.WriteString("idle ")
		}

		numIdle := len(hmState.HostsByState[core.HostAgentStateConnectedIdle])
		numBusy := len(hmState.HostsByState[core.HostAgentStateConnectedBusy])
		numOther := hmState.NumHosts - numIdle - numBusy

		sb.WriteString(getStatuslineNumStr("🖳", numIdle, "green"))
		sb.WriteString(" ")
		sb.WriteString(getStatuslineNumStr("🖳", numBusy, "orange"))
		sb.WriteString(" ")
		sb.WriteString(getStatuslineNumStr("🖳", numOther, "red"))
		sb.WriteString(" ")
		sb.WriteString(getStatuslineNumStr("🖳", hmState.NumUnused, "gray"))

		mv.statusLine.SetText(sb.String())
	})
}

func getStatuslineNumStr(icon string, num int, color string) string {
	mod := "-"
	if num > 0 {
		mod = "b"
	}

	return fmt.Sprintf("[%s:-:%s]%s %.2d[-:-:-]", color, mod, icon, num)
}

func (mv *MainView) updateTableHeader(msgs []core.LogMsg) (colNames []string) {
	colNames = []string{"time", "message", "source"}
	// TODO: add all context tags from the available logs

	// TODO: clear or update table from the prev state

	// Add header
	for i, colName := range colNames {
		mv.logsTable.SetCell(
			0, i,
			newTableCellHeader(colName),
		)
	}

	return colNames
}

func (mv *MainView) ApplyLogs(resp *core.LogResp) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.curLogResp = resp

		histogramData := make(map[int]int, len(resp.MinuteStats))
		for k, v := range resp.MinuteStats {
			histogramData[int(k)] = v.NumMsgs
		}

		mv.histogram.SetData(histogramData)

		// TODO: when we implement loading _more_ logs for the same query,
		// we shouldn't clear table or selection
		mv.logsTable.Clear()

		colNames := mv.updateTableHeader(resp.Logs)
		// Add all available logs
		for i, rowIdx := len(resp.Logs)-1, 1; i >= 0; i, rowIdx = i-1, rowIdx+1 {
			msg := resp.Logs[i]

			mv.logsTable.SetCell(
				rowIdx, 0,
				newTableCellLogmsg(msg.Time.Format(logsTableTimeLayout)).SetTextColor(tcell.ColorYellow),
			)

			mv.logsTable.SetCell(
				rowIdx, 1,
				newTableCellLogmsg(msg.Msg),
			)

			for i, colName := range colNames[2:] {
				mv.logsTable.SetCell(
					rowIdx, 2+i,
					newTableCellLogmsg(msg.Context[colName]),
				)
			}

			//msg.
		}

		mv.logsTable.Select(0, 0)
		mv.logsTable.ScrollToBeginning()

		mv.bumpTimeRange()
	})
}

func newTableCellHeader(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorYellow).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
}

func newTableCellLogmsg(text string) *tview.TableCell {
	return tview.NewTableCell(text).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft)
}

/*

	mv.bottomForm = tview.NewForm().
		AddButton("Place order", func() {
			fmt.Println("Place order")
			//msv := NewMarketSelectorView(mv, &MarketSelectorParams{
			//Title: "Place order on which market?",
			//OnSelected: func(marketID common.MarketID) bool {
			//pov := NewPlaceOrderView(mv, &PlaceOrderViewParams{
			//Market: mv.marketDescrByID[marketID],
			//})

			//pov.Show()
			//return true
			//},
			//})
			//msv.Show()
		}).
		AddButton("Cancel order", func() {
			//msv := NewMarketSelectorView(mv, &MarketSelectorParams{
			//Title: "Cancel order on which market?",
			//OnSelected: func(marketID common.MarketID) bool {

			//// Even though we're in the UI loop right now, we can't invoke
			//// FocusOrdersList right here, because when OnSelected returns, we
			//// hide the modal window, and focus will be moved back to the bottom
			//// menu. We need to call FocusOrdersList _after_ that.
			//mv.params.App.QueueUpdateDraw(func() {
			//mv.marketViewsByID[marketID].FocusOrdersList(
			//func(order common.PrivateOrder) {
			//// TODO: confirm
			//mv.params.OnCancelOrderRequest(common.CancelOrderParams{
			//MarketID: marketID,
			//OrderID:  order.ID,
			//})
			//mv.params.App.SetFocus(mv.bottomForm)
			//},
			//func() {
			//mv.params.App.SetFocus(mv.bottomForm)
			//},
			//)
			//})
			//return true
			//},
			//})
			//msv.Show()
		}).
		AddButton("Quit", func() {
			params.App.Stop()
		}).
		AddButton("I said quit", func() {
			params.App.Stop()
		})

	mainFlex.AddItem(mv.bottomForm, 3, 0, false)

*/

func (mv *MainView) setTimeRange(from, to TimeOrDur) {
	if from.IsZero() {
		// TODO: maybe better error handling
		panic("from can't be zero")
	}

	mv.from = from
	mv.to = to

	mv.bumpTimeRange()

	rangeDur := mv.actualTo.Sub(mv.actualFrom)

	var timeStr string
	if !mv.to.IsZero() {
		timeStr = fmt.Sprintf("%s to %s (%s)", mv.from, mv.to, formatDuration(rangeDur))
	} else if mv.from.IsAbsolute() {
		timeStr = fmt.Sprintf("%s to now (%s)", mv.from, formatDuration(rangeDur))
	} else {
		timeStr = fmt.Sprintf("last %s", TimeOrDur{Dur: -mv.from.Dur})
	}

	mv.timeLabel.SetText(timeStr)
	mv.topFlex.ResizeItem(mv.timeLabel, len(timeStr), 0)

}

// bumpTimeRange only does something useful if the time is relative to current time.
func (mv *MainView) bumpTimeRange() {
	if mv.from.IsZero() {
		panic("should never be here")
	}

	// Since relative durations are relative to current time, only negative values are
	// meaningful, so if it's positive, reverse it.

	if !mv.from.IsAbsolute() && mv.from.Dur > 0 {
		mv.from.Dur = -mv.from.Dur
	}

	if !mv.to.IsAbsolute() && mv.to.Dur > 0 {
		mv.to.Dur = -mv.to.Dur
	}

	mv.actualFrom = mv.from.AbsoluteTime(time.Now())

	if !mv.to.IsZero() {
		mv.actualTo = mv.to.AbsoluteTime(time.Now())
	} else {
		mv.actualTo = time.Now()
	}

	// Snap both actualFrom and actualTo to the 1m grid, rounding forward.
	mv.actualFrom = mv.actualFrom.Truncate(1 * time.Minute).Add(1 * time.Minute)
	mv.actualTo = mv.actualTo.Truncate(1 * time.Minute).Add(1 * time.Minute)

	// If from is after than to, swap them.
	if mv.actualFrom.After(mv.actualTo) {
		mv.actualFrom, mv.actualTo = mv.actualTo, mv.actualFrom
	}

	// Also update the histogram
	mv.histogram.SetRange(int(mv.actualFrom.Unix()), int(mv.actualTo.Unix()))
}

func (mv *MainView) SetTimeRange(from, to TimeOrDur) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.setTimeRange(from, to)
	})
}

func (mv *MainView) setHostsFilter(s string) {
	mv.hostsFilter = s
}

func (mv *MainView) doQuery() {
	mv.params.OnLogQuery(core.QueryLogsParams{
		From:  mv.actualFrom,
		To:    mv.actualTo,
		Query: mv.query,
	})
}

func (mv *MainView) DoQuery() {
	mv.params.App.QueueUpdateDraw(func() {
		mv.doQuery()
	})
}

func formatDuration(dur time.Duration) string {
	ret := dur.String()

	// Strip useless suffix
	if strings.HasSuffix(ret, "h0m0s") {
		return ret[:len(ret)-4]
	} else if strings.HasSuffix(ret, "m0s") {
		return ret[:len(ret)-2]
	}

	return ret
}

type MessageboxParams struct {
	Buttons         []string
	OnButtonPressed func(label string, idx int)
}

func (mv *MainView) showMessagebox(
	msgID, title, message string, params *MessageboxParams,
) {
	var msgvErr *MessageView

	if params == nil {
		params = &MessageboxParams{
			Buttons: []string{"OK"},
			OnButtonPressed: func(label string, idx int) {
				msgvErr.Hide()
			},
		}
	}

	msgvErr = NewMessageView(mv, &MessageViewParams{
		MessageID:       msgID,
		Title:           title,
		Message:         message,
		Buttons:         params.Buttons,
		OnButtonPressed: params.OnButtonPressed,

		//Width: 60,
		Width:  120, // TODO: from params
		Height: 20,
	})
	msgvErr.Show()
}

func (mv *MainView) ShowMessagebox(
	msgID, title, message string, params *MessageboxParams,
) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.showMessagebox(msgID, title, message, params)
	})
}

func (mv *MainView) HideMessagebox(msgID string) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.hideModal(pageNameMessage + msgID)
	})
}

func (mv *MainView) showModal(name string, primitive tview.Primitive, width, height int) {
	mv.modalsFocusStack = append(mv.modalsFocusStack, mv.params.App.GetFocus())

	// Returns a new primitive which puts the provided primitive in the center and
	// sets its size to the given width and height.
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewGrid().
			SetColumns(0, width, 0).
			SetRows(0, height, 0).
			AddItem(p, 1, 1, 1, 1, 0, 0, true)
	}

	mv.rootPages.AddPage(name, modal(primitive, width, height), true, true)

	mv.params.App.SetFocus(primitive)
}

func (mv *MainView) hideModal(name string) {
	mv.rootPages.RemovePage(name)
	l := len(mv.modalsFocusStack)
	mv.params.App.SetFocus(mv.modalsFocusStack[l-1])
	mv.modalsFocusStack = mv.modalsFocusStack[:l-1]
}
