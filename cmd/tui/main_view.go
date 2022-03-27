package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dimonomid/nerdlog/core"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const logsTableTimeLayout = "Jan02 15:04:05.000"

type MainViewParams struct {
	App *tview.Application

	// OnLogQuery is called by MainView whenever the user submits a query to get
	// logs.
	OnLogQuery OnLogQueryCallback
}

type MainView struct {
	params    MainViewParams
	rootPages *tview.Pages
	logsTable *tview.Table

	queryInput *tview.InputField
	cmdInput   *tview.InputField

	histogram *Histogram

	statusLine *tview.TextView

	// from, to represent the selected time range
	from, to time.Time

	// actualFrom, actualTo is like from and to, but they can't be zero.
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
}

type OnLogQueryCallback func(core.QueryLogsParams)

func NewMainView(params *MainViewParams) *MainView {
	mv := &MainView{
		params: *params,
	}

	mv.rootPages = tview.NewPages()

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	mv.queryInput = tview.NewInputField()
	mv.queryInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			// TODO: remove it from here
			mv.setTimeRange(time.Now().Add(-5*time.Hour), time.Time{})

			mv.params.OnLogQuery(core.QueryLogsParams{
				From:  mv.from,
				Query: mv.queryInput.GetText(),
			})
		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.logsTable)
		}
	})

	mainFlex.AddItem(mv.queryInput, 1, 0, false)

	mv.histogram = NewHistogram()
	mv.histogram.SetBinSize(60) // 1 minute
	mv.histogram.SetXFormatter(func(v int) string {
		t := time.Unix(int64(v), 0)
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
		}

		return event
	})

	// TODO: once tableview fixed, use SetFixed(1, 1)
	// (there's an issue with going to the very top using "g")
	mv.logsTable.SetFixed(1, 1)
	mv.logsTable.Select(0, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			mv.params.App.Stop()
		}
		if key == tcell.KeyEnter {
			//mv.logsTable.SetSelectable(true, true)
		}
		if key == tcell.KeyBacktab {
			mv.params.App.SetFocus(mv.queryInput)
		}
	}).SetSelectedFunc(func(row int, column int) {
		// TODO: show the full message
		//mv.logsTable.GetCell(row, column).SetTextColor(tcell.ColorRed)
		//mv.logsTable.SetSelectable(false, false)
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

	mainFlex.AddItem(mv.logsTable, 0, 1, true)

	mv.statusLine = tview.NewTextView()
	mv.statusLine.SetScrollable(false).SetDynamicColors(true)

	mainFlex.AddItem(mv.statusLine, 1, 0, false)

	mv.cmdInput = tview.NewInputField()

	mainFlex.AddItem(mv.cmdInput, 1, 0, false)

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
		// TODO: handle resp.Err, maybe just a dialog
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

		mv.updateHistogramTimeRange()
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

func (mv *MainView) setTimeRange(from, to time.Time) {
	mv.from = from
	mv.to = to

	mv.updateHistogramTimeRange()
}

func (mv *MainView) SetTimeRange(from, to time.Time) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.setTimeRange(from, to)
	})
}

func (mv *MainView) updateHistogramTimeRange() {
	var fromUnix, toUnix int

	mv.actualFrom, mv.actualTo = mv.from, mv.to

	if mv.actualFrom.IsZero() {
		mv.actualFrom = mv.statsFrom
	}

	if mv.actualFrom.IsZero() {
		mv.actualFrom = time.Now()
	}

	mv.actualFrom = mv.actualFrom.Truncate(1 * time.Minute).Add(1 * time.Minute)

	fromUnix = int(mv.actualFrom.Unix())

	if mv.actualTo.IsZero() {
		mv.actualTo = mv.statsTo
	}

	if mv.actualTo.IsZero() {
		mv.actualTo = time.Now()
	}

	mv.actualTo = mv.actualTo.Truncate(1 * time.Minute).Add(1 * time.Minute)

	toUnix = int(mv.actualTo.Unix())

	mv.histogram.SetRange(fromUnix, toUnix)
}
