package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dimonomid/nerdlog/clhistory"
	"github.com/dimonomid/nerdlog/cmd/nerdlog-tui/ui"
	"github.com/dimonomid/nerdlog/core"
	"github.com/dimonomid/nerdlog/log"
	"github.com/gdamore/tcell/v2"
	"github.com/juju/errors"
	"github.com/rivo/tview"
)

const logsTableTimeLayout = "Jan02 15:04:05.000"

const (
	pageNameMessage         = "message"
	pageNameEditQueryParams = "edit_query_params"
	pageNameRowDetails      = "row_details"
	pageNameColumnDetails   = "column_details"
	pageNameTextView        = "text_view"
)

const (
	// rowIdxLoadOlder is the index of the row acting as a button to load more (older) logs
	rowIdxLoadOlder = 1
)

const histogramBinSize = 60 // 1 minute

type MainViewParams struct {
	App *tview.Application

	Options *OptionsShared

	// OnLogQuery is called by MainView whenever the user submits a query to get
	// logs.
	OnLogQuery OnLogQueryCallback

	OnLStreamsChange OnLStreamsChange

	OnDisconnectRequest OnDisconnectRequest
	OnReconnectRequest  OnReconnectRequest

	// TODO: support command history
	OnCmd OnCmdCallback

	CmdHistory   *clhistory.CLHistory
	QueryHistory *clhistory.CLHistory

	Logger *log.Logger
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

	menuDropdown *ui.DropDown

	queryEditView *QueryEditView

	// overlayMsgView is nil if there's no overlay msg.
	overlayMsgView            *MessageView
	overlayText               string
	overlaySpinner            rune
	overlayMsgViewIsMinimized bool

	// focusedBeforeCmd is a primitive which was focused before cmdInput was
	// focused. Once the user is done editing command, focusedBeforeCmd
	// normally resumes focus.
	focusedBeforeCmd tview.Primitive

	histogram *Histogram

	statusLineLeft  *tview.TextView
	statusLineRight *tview.TextView

	lstreamsSpec string

	// from, to represent the selected time range
	from, to TimeOrDur

	// query is the effective search query
	query string

	// actualFrom, actualTo represent the actual time range resolved from from
	// and to, and they both can't be zero.
	//
	// NOTE: don't use actualTo for the queries (QueryLogsParams); use
	// actualToForQuery instead, see below.
	actualFrom, actualTo time.Time

	// selectQuery is the effective SelectQuery
	selectQuery *SelectQueryParsed

	// actualToForQuery is similar to actualTo, but if the "to" was at zero
	// value, then actualToForQuery will be zero value too. It's suitable for the
	// use in queries (QueryLogsParams); and it must be used instead of actualTo,
	// because when requesting latest logs, actualTo is actually in the future
	// and if we pass a timestamp in the future to nerdlog_agent.sh, it will
	// uselessly try to update the cache (the timestamp -> linenumber mapping),
	// trying to find this non-existing future timestamp there.
	actualToForQuery time.Time

	// existingTagNames is a list of all tag names that exist in currently
	// queried logs (regardless of whether those columns exist in the UI).
	existingTagNames map[string]struct{}

	// When doQueryParamsOnceConnected is not nil, it means that whenever we get
	// a new status update (ApplyHMState gets called), if Connected is true
	// there, we'll call doQuery().
	doQueryParamsOnceConnected *doQueryParams

	// If sendLStreamsChangeOnNextQuery, then the next time the user wants to
	// make a query (just the awk query, without the timeframe and logstreams),
	// we'll first update the logstreams, and only then make the query.
	sendLStreamsChangeOnNextQuery bool

	curHMState *core.LStreamsManagerState
	curLogResp *core.LogRespTotal
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

type CmdOpts struct {
	// If Internal is true, it means the user didn't actually type the command,
	// it was generated using some other way; so e.g. it shouldn't be added to the
	// command line history.
	Internal bool
}

type OnLogQueryCallback func(params core.QueryLogsParams)
type OnLStreamsChange func(lstreamsSpec string) error
type OnDisconnectRequest func()
type OnReconnectRequest func()
type OnCmdCallback func(cmd string, opts CmdOpts)

var (
	queryInputStaleMatch = tcell.Style{}.
				Background(tcell.ColorBlue).
				Foreground(tcell.ColorWhite).
				Bold(true)

	queryInputStaleMismatch = tcell.Style{}.
				Background(tcell.ColorDarkRed).
				Foreground(tcell.ColorWhite).
				Bold(true)

	menuUnselected = tcell.Style{}.
			Background(tcell.ColorBlue).
			Foreground(tcell.ColorWhite).
			Bold(true)

	menuSelected = tcell.Style{}.
			Background(tcell.ColorWhite).
			Foreground(tcell.ColorBlue).
			Bold(true)

	cmdLineCommand = tcell.Style{}.
			Background(tcell.ColorBlue).
			Foreground(tcell.ColorWhite).
			Bold(false)

	cmdLineMsgInfo = tcell.Style{}.
			Background(tcell.ColorBlue).
			Foreground(tcell.ColorWhite).
			Bold(false)

	cmdLineMsgWarn = tcell.Style{}.
			Background(tcell.ColorBlue).
			Foreground(tcell.ColorLime).
			Bold(true)

	cmdLineMsgErr = tcell.Style{}.
			Background(tcell.ColorBlue).
			Foreground(tcell.ColorYellow).
			Bold(false)
)

func NewMainView(params *MainViewParams) *MainView {
	params.Logger = params.Logger.WithNamespaceAppended("MainView")

	mv := &MainView{
		params: *params,
	}

	var err error
	mv.selectQuery, err = ParseSelectQuery(DefaultSelectQuery)
	if err != nil {
		panic(err.Error())
	}

	mv.rootPages = tview.NewPages()

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	mv.queryInput = tview.NewInputField()
	mv.queryInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			mv.setQuery(mv.queryInput.GetText())
			mv.bumpTimeRange(false)

			if mv.sendLStreamsChangeOnNextQuery {
				// Before making a query, we need to update the logstreams first.

				mv.sendLStreamsChangeOnNextQuery = false
				if err := mv.params.OnLStreamsChange(mv.lstreamsSpec); err != nil {
					// It shouldn't happen really, since if we already had some mv.lstreamsSpec,
					// it means it must have already passed the checks and can't be invalid,
					// but just in case, handle this error as well.
					mv.showMessagebox(
						"err",
						"Broken logstreams filter",
						fmt.Sprintf("Resetting the logstreams filter, since the current one '%q' is wrong: %s", mv.lstreamsSpec, err.Error()),
						&MessageboxParams{
							BackgroundColor: tcell.ColorDarkRed,
						},
					)
					mv.setLStreams("")
					return nil
				}

				// Now that the logstreams are updated, schedule the query once the
				// connections are ready.
				mv.doQueryParamsOnceConnected = &doQueryParams{}
			} else {
				// All the logstreams are supposed to be ready, so just do the query
				// right away.
				mv.doQuery(doQueryParams{})
			}

			mv.queryInputApplyStyle()
			return nil

		case tcell.KeyEsc:
			//if mv.queryInput.GetText() != mv.query {
			//mv.queryInput.SetText(mv.query)
			//mv.queryInputApplyStyle()
			//}
			mv.params.App.SetFocus(mv.logsTable)
			return nil

		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.queryEditBtn)
			return nil

		case tcell.KeyBacktab:
			mv.params.App.SetFocus(mv.logsTable)
			return nil

		case tcell.KeyCtrlP, tcell.KeyUp, tcell.KeyCtrlN, tcell.KeyDown:
			var item clhistory.Item
			qf := QueryFull{
				Query: mv.queryInput.GetText(),
			}
			cmd := qf.MarshalShellCmd()

			for {
				var hasMore bool
				if event.Key() == tcell.KeyCtrlP || event.Key() == tcell.KeyUp {
					item, hasMore = mv.params.QueryHistory.Prev(cmd)
				} else {
					item, hasMore = mv.params.QueryHistory.Next(cmd)
				}

				var tmp QueryFull
				if err := tmp.UnmarshalShellCmd(item.Str); err != nil {
					mv.showMessagebox("err", "Broken query history", err.Error(), nil)
					return nil
				}

				if (tmp.Query != "" && tmp.Query != qf.Query) || !hasMore {
					// Either we found a different value for this field, or ran out of
					// history. Set this value in the original QueryFull, and use it.
					qf.Query = tmp.Query
					break
				}
			}

			mv.queryInput.SetText(qf.Query)
			return nil

		case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2,
			tcell.KeyDelete, tcell.KeyCtrlD,
			tcell.KeyCtrlW, tcell.KeyCtrlU, tcell.KeyCtrlK:

			mv.params.QueryHistory.Reset()
		}

		return event
	})

	mv.queryInput.SetChangedFunc(func(text string) {
		mv.queryInputApplyStyle()
	})

	mv.queryInputApplyStyle()

	mv.queryEditBtn = tview.NewButton("Edit")
	mv.queryEditBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.menuDropdown)
		case tcell.KeyBacktab:
			mv.params.App.SetFocus(mv.queryInput)
			return nil

		case tcell.KeyEsc:
			mv.params.App.SetFocus(mv.logsTable)

		case tcell.KeyRune:
			switch event.Rune() {
			case ':':
				mv.focusCmdline()
				return nil

			case 'i', 'a':
				mv.params.App.SetFocus(mv.queryInput)
				return nil
			}
		}

		return event
	})
	mv.queryEditBtn.SetSelectedFunc(func() {
		mv.openQueryEditView()
	})

	queryLabel := tview.NewTextView()
	queryLabel.SetScrollable(false).SetText("Query:")

	mv.timeLabel = tview.NewTextView()
	mv.timeLabel.SetScrollable(false)

	mv.menuDropdown = ui.NewDropDown()
	mv.menuDropdown.SetOptions(getMainMenuTitles(), nil)
	mv.menuDropdown.SetListStyles(menuUnselected, menuSelected)
	mv.menuDropdown.SetTextOptions(" ", " ", " ", " ", " Menu ")
	mv.menuDropdown.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if mv.menuDropdown.IsListOpen() {
				mv.menuDropdown.SetCurrentOption(-1)
				mv.menuDropdown.CloseList(mv.setFocus)
			}
			mv.params.App.SetFocus(mv.histogram)
			return nil
		case tcell.KeyBacktab:
			if mv.menuDropdown.IsListOpen() {
				mv.menuDropdown.SetCurrentOption(-1)
				mv.menuDropdown.CloseList(mv.setFocus)
			}
			mv.params.App.SetFocus(mv.queryEditBtn)
			return nil

		case tcell.KeyEsc:
			if mv.menuDropdown.IsListOpen() {
				mv.menuDropdown.SetCurrentOption(-1)
				mv.menuDropdown.CloseList(mv.setFocus)
			} else {
				mv.params.App.SetFocus(mv.logsTable)
			}
			return nil

		case tcell.KeyEnter:
			if mv.menuDropdown.IsListOpen() {
				list := mv.menuDropdown.GetList()
				idx := list.GetCurrentItem()

				mv.menuDropdown.SetCurrentOption(-1)
				mv.menuDropdown.CloseList(mv.setFocus)

				// NOTE: the CloseList MUST be called before invoking the handler,
				// because if the handler calls e.g. showMessagebox which remembers
				// which primitive is focused, then without calling CloseList first,
				// the list would be focused, and when the messagebox is finally
				// closed, focusing this list again means getting to a focus trap.

				mainMenu[idx].Handler(mv)

				return nil
			}

		case tcell.KeyRune:
			list := mv.menuDropdown.GetList()
			if mv.menuDropdown.IsListOpen() {

				switch event.Rune() {
				case 'j', 'l':
					idx := list.GetCurrentItem()
					idx++
					if idx >= list.GetItemCount() {
						idx = 0
					}
					list.SetCurrentItem(idx)
				case 'k', 'h':
					idx := list.GetCurrentItem()
					idx--
					if idx < 0 {
						idx = list.GetItemCount() - 1
					}
					list.SetCurrentItem(idx)
				case 'g':
					list.SetCurrentItem(0)
				case 'G':
					list.SetCurrentItem(list.GetItemCount() - 1)
				}

			} else {
				switch event.Rune() {
				case ':':
					mv.focusCmdline()
					return nil

				case 'i', 'a':
					mv.params.App.SetFocus(mv.queryInput)
					return nil

				case 'j':
					mv.menuDropdown.OpenList(mv.setFocus)
					list.SetCurrentItem(0)
					return nil
				case 'k':
					mv.menuDropdown.OpenList(mv.setFocus)
					list.SetCurrentItem(list.GetItemCount() - 1)
					return nil
				}
			}

			return nil
		}

		return event
	})

	mv.topFlex = tview.NewFlex().SetDirection(tview.FlexColumn)
	mv.topFlex.
		AddItem(queryLabel, 6, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.queryInput, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(mv.timeLabel, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.queryEditBtn, 6, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.menuDropdown, 6, 0, false)

	mainFlex.AddItem(mv.topFlex, 1, 0, true)

	mv.histogram = NewHistogram()
	mv.histogram.SetBinSize(histogramBinSize) // 1 minute
	mv.histogram.SetXFormatter(func(v int) string {
		tz := mv.params.Options.GetTimezone()

		t := time.Unix(int64(v), 0).In(tz)
		if t.Hour() == 0 && t.Minute() == 0 {
			return t.In(tz).Format("[yellow]Jan02[-]")
		}
		return t.In(tz).Format("15:04")
	})
	mv.histogram.SetCursorFormatter(func(from int, to *int, width int) string {
		tz := mv.params.Options.GetTimezone()
		fromTime := time.Unix(int64(from), 0).In(tz)

		if to == nil {
			return fromTime.In(tz).Format("Jan02 15:04")
		}

		toTime := time.Unix(int64(*to), 0).In(tz)

		return fmt.Sprintf(
			"%s - %s (%s)",
			fromTime.In(tz).Format("Jan02 15:04"),
			toTime.In(tz).Format("Jan02 15:04"),
			strings.TrimSuffix(toTime.Sub(fromTime).String(), "0s"),
		)
	})
	mv.histogram.SetXMarker(func(from, to int, numChars int) []int {
		tz := mv.params.Options.GetTimezone()
		return getXMarksForHistogram(tz, from, to, numChars)
	})
	mv.histogram.SetDataBinsSnapper(snapDataBinsInChartDot)
	mv.histogram.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			mv.params.App.SetFocus(mv.logsTable)
			return nil
		case tcell.KeyBacktab:
			mv.params.App.SetFocus(mv.menuDropdown)
			return nil

		case tcell.KeyEsc:
			if !mv.histogram.IsSelectionActive() {
				mv.params.App.SetFocus(mv.logsTable)
				return nil
			}

		case tcell.KeyRune:
			switch event.Rune() {
			case ':':
				mv.focusCmdline()
				return nil

			case 'i', 'a':
				mv.params.App.SetFocus(mv.queryInput)
				return nil
			}
		}

		return event
	})
	mv.histogram.SetSelectedFunc(func(from, to int) {
		tz := mv.params.Options.GetTimezone()

		fromTime := TimeOrDur{
			Time: time.Unix(int64(from), 0).In(tz),
		}

		toTime := TimeOrDur{
			Time: time.Unix(int64(to), 0).In(tz),
		}

		mv.setTimeRange(fromTime, toTime)
		mv.doQuery(doQueryParams{})
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

		case tcell.KeyEsc:
			if mv.overlayMsgView != nil && mv.overlayMsgViewIsMinimized {
				mv.makeOverlayVisible()
				mv.bumpOverlay()
			}

		case tcell.KeyRune:
			switch event.Rune() {
			case ':':
				mv.focusCmdline()
				return nil

			case 'i', 'a':
				mv.params.App.SetFocus(mv.queryInput)
				return nil
			}
		}

		return event
	})

	mv.logsTable.Select(0, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			//mv.logsTable.SetSelectable(true, true)
		}
		if key == tcell.KeyTab {
			mv.params.App.SetFocus(mv.queryInput)
		}
		if key == tcell.KeyBacktab {
			mv.params.App.SetFocus(mv.histogram)
		}
	}).SetSelectedFunc(func(row int, column int) {
		if row == rowIdxLoadOlder {
			// Request to load more (older) logs

			// Do the query to core
			mv.params.OnLogQuery(core.QueryLogsParams{
				From:  mv.actualFrom,
				To:    mv.actualToForQuery,
				Query: mv.query,

				LoadEarlier: true,
			})

			// Update the cell text
			mv.logsTable.SetCell(
				rowIdxLoadOlder, 0,
				newTableCellButton("... loading ..."),
			)
			return
		}

		// "Click" on a data cell: show details

		firstCell := mv.logsTable.GetCell(row, 0)
		msg := firstCell.GetReference().(core.LogMsg)

		existingNamesSet := map[string]struct{}{
			FieldNameTime:    {},
			FieldNameMessage: {},
		}
		for key := range msg.Context {
			existingNamesSet[key] = struct{}{}
		}

		rdv := NewRowDetailsView(mv, &RowDetailsViewParams{
			DoneFunc:         mv.applyQueryEditData,
			Data:             mv.getQueryFull(),
			ExistingNamesSet: existingNamesSet,
			Msg:              &msg,
		})
		rdv.Show()
	}).SetSelectionChangedFunc(func(row, column int) {
		mv.bumpStatusLineRight()
	})

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

	mv.statusLineLeft = tview.NewTextView()
	mv.statusLineLeft.SetScrollable(false).SetDynamicColors(true)

	mv.statusLineRight = tview.NewTextView()
	mv.statusLineRight.SetTextAlign(tview.AlignRight).SetScrollable(false).SetDynamicColors(true)

	statusLineFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	statusLineFlex.
		AddItem(mv.statusLineLeft, 0, 1, false).
		AddItem(nil, 1, 0, false).
		AddItem(mv.statusLineRight, 30, 0, true)

	mainFlex.AddItem(statusLineFlex, 1, 0, false)

	mv.cmdInput = tview.NewInputField()
	mv.cmdInput.SetFieldStyle(cmdLineCommand)
	mv.cmdInput.SetChangedFunc(func(text string) {
		if text == "" {
			mv.params.App.SetFocus(mv.focusedBeforeCmd)
		}
	})

	mv.cmdInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		cmd := mv.cmdInput.GetText()
		// Remove the ":" prefix
		cmd = cmd[1:]

		switch event.Key() {
		case tcell.KeyCtrlP, tcell.KeyUp:
			item, _ := mv.params.CmdHistory.Prev(cmd)
			mv.cmdInput.SetText(":" + item.Str)
			return nil

		case tcell.KeyCtrlN, tcell.KeyDown:
			item, _ := mv.params.CmdHistory.Next(cmd)
			mv.cmdInput.SetText(":" + item.Str)
			return nil
		}

		mv.params.CmdHistory.Reset()

		return event
	})

	mv.cmdInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			cmd := mv.cmdInput.GetText()

			// Remove the ":" prefix
			cmd = cmd[1:]

			if cmd != "" {
				mv.params.OnCmd(cmd, CmdOpts{})
			} else {
				// Similarly to zsh, make it so that an empty command causes history to
				// be reloaded.  TODO: maybe make it so that we reload it after any
				// command, actually.
				mv.params.CmdHistory.Load()
			}

		case tcell.KeyEsc:
		// Gonna just stop editing it
		default:
			// Ignore it
			return
		}

		mv.cmdInput.SetText("")
		mv.params.CmdHistory.Reset()
	})

	mainFlex.AddItem(mv.cmdInput, 1, 0, false)

	mv.queryEditView = NewQueryEditView(mv, &QueryEditViewParams{
		DoneFunc: mv.applyQueryEditData,
		History:  mv.params.QueryHistory,
	})

	mv.rootPages.AddPage("mainFlex", mainFlex, true, true)

	// Set default time range, just to have anything there.
	from, to := TimeOrDur{Dur: -1 * time.Hour}, TimeOrDur{}
	mv.setTimeRange(from, to)

	go mv.run()

	return mv
}

func (mv *MainView) focusCmdline() {
	mv.cmdInput.SetFieldStyle(cmdLineCommand)
	mv.cmdInput.SetText(":")
	mv.focusedBeforeCmd = mv.params.App.GetFocus()
	mv.params.App.SetFocus(mv.cmdInput)
}

type nlMsgLevel string

const (
	nlMsgLevelInfo nlMsgLevel = "info"
	nlMsgLevelWarn nlMsgLevel = "warn"
	nlMsgLevelErr  nlMsgLevel = "err"
)

func (mv *MainView) printMsg(s string, level nlMsgLevel) {
	// If the commandline is focused, then don't print anything since it would mess
	// with the current input.
	//
	// TODO: keep some history of the messages that were printed, so that we can at least
	// check later what exactly has happened.
	if mv.cmdInput.HasFocus() {
		// TODO: still indicate _somehow_ that a message has happened.
		return
	}

	style := cmdLineMsgInfo
	switch level {
	case nlMsgLevelInfo:
		style = cmdLineMsgInfo
	case nlMsgLevelWarn:
		style = cmdLineMsgWarn
	case nlMsgLevelErr:
		style = cmdLineMsgErr
	}

	mv.cmdInput.SetFieldStyle(style)
	mv.cmdInput.SetText(s)
}

func (mv *MainView) run() {
	ticker := time.NewTicker(250 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			needDraw := false
			mv.params.App.QueueUpdate(func() {
				needDraw = mv.tick()
			})

			if needDraw {
				mv.params.App.Draw()
			}
		}
	}
}

func (mv *MainView) tick() (needDraw bool) {
	if mv.overlayMsgView != nil {
		switch mv.overlaySpinner {
		case '-':
			mv.overlaySpinner = '\\'
		case '\\':
			mv.overlaySpinner = '|'
		case '|':
			mv.overlaySpinner = '/'
		case '/':
			mv.overlaySpinner = '-'
		}

		mv.bumpOverlay()

		needDraw = true
	}

	return needDraw
}

func (mv *MainView) bumpOverlay() {
	// If overlay message isn't minimized by the user, update it;
	// otherwise, print a message in the command line.
	text := string(mv.overlaySpinner) + " " + mv.overlayText
	if !mv.overlayMsgViewIsMinimized {
		mv.overlayMsgView.textView.SetText(text)
	} else {
		mv.printOverlayMsgInCmdline(text)
	}
}

func (mv *MainView) queryInputApplyStyle() {
	style := queryInputStaleMatch
	if mv.queryInput.GetText() != mv.query {
		style = queryInputStaleMismatch
	}

	mv.queryInput.SetFieldStyle(style)
}

func (mv *MainView) applyQueryEditData(data QueryFull, dqp doQueryParams) error {
	tz := mv.params.Options.GetTimezone()

	ftr, err := ParseFromToRange(tz, data.Time)
	if err != nil {
		return errors.Annotatef(err, "time")
	}

	sqp, err := ParseSelectQuery(data.SelectQuery)
	if err != nil {
		return errors.Annotatef(err, "select query")
	}

	mv.setQuery(data.Query)
	mv.setTimeRange(ftr.From, ftr.To)

	mv.params.Logger.Infof("Applying lstreams: %s", data.LStreams)
	err = mv.params.OnLStreamsChange(data.LStreams)
	if err != nil {
		return errors.Annotate(err, "lstreams")
	}

	mv.setSelectQuery(sqp)

	mv.setLStreams(data.LStreams)

	mv.bumpStatusLineLeft()

	mv.queryInputApplyStyle()

	// We don't actually initiate a query here, but we set this
	// doQueryParamsOnceConnected flag, so that next time we receive a status
	// update (which will happen since we called OnLStreamsChange above), if
	// Connected is true there, we'll do the query.
	mv.doQueryParamsOnceConnected = &dqp

	mv.sendLStreamsChangeOnNextQuery = false

	return nil
}

func (mv *MainView) GetUIPrimitive() tview.Primitive {
	return mv.rootPages
}

func (mv *MainView) applyHMState(lsmanState *core.LStreamsManagerState) {
	mv.params.Logger.Verbose1f("Applying HM state: %+v", lsmanState)

	mv.curHMState = lsmanState
	var overlayMsg string

	if !mv.curHMState.Connected && !mv.curHMState.NoMatchingLStreams {
		var sb strings.Builder

		sb.WriteString("Connecting to lstreams...")

		for logstream, connDetails := range lsmanState.ConnDetailsByLStream {
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("%s: %s", logstream, connDetails.Err))
		}

		overlayMsg = sb.String()
	} else if mv.curHMState.Busy {
		var sb strings.Builder

		sb.WriteString("Updating search results...")

		// If we have info about lstreams busy stage, show the slowest one.
		if len(lsmanState.BusyStageByLStream) > 0 {
			type lstreamWBusyStage struct {
				logstream string
				stage     core.BusyStage
			}

			vs := make([]lstreamWBusyStage, 0, len(lsmanState.BusyStageByLStream))
			for logstream, stage := range lsmanState.BusyStageByLStream {
				vs = append(vs, lstreamWBusyStage{
					logstream: logstream,
					stage:     stage,
				})
			}

			sort.Slice(vs, func(i, j int) bool {
				if vs[i].stage.Num != vs[j].stage.Num {
					return vs[i].stage.Num < vs[j].stage.Num
				}

				if vs[i].stage.Percentage != vs[j].stage.Percentage {
					return vs[i].stage.Percentage < vs[j].stage.Percentage
				}

				return vs[i].logstream < vs[j].logstream
			})

			slowest := vs[0]

			sb.WriteString("\n[lightgray]")
			sb.WriteString(slowest.stage.Title)
			if slowest.stage.Percentage != 0 {
				sb.WriteString(fmt.Sprintf(" (%d%%)", slowest.stage.Percentage))
			}

			sb.WriteString(" - " + slowest.logstream)
			sb.WriteString("[-]")
		}

		overlayMsg = sb.String()
	}

	if overlayMsg != "" {
		// Need to show or update overlay message.
		mv.overlayText = overlayMsg

		if mv.overlayMsgView == nil {
			mv.makeOverlayVisible()

			mv.overlaySpinner = '-'
		}

		mv.bumpOverlay()

	} else if mv.overlayMsgView != nil {
		if !mv.overlayMsgViewIsMinimized {
			// Need to hide overlay message.
			mv.hideOverlayMsgBox()
		}

		mv.overlayMsgViewIsMinimized = false
		mv.overlayMsgView = nil
		mv.overlayText = ""
	}

	mv.bumpStatusLineLeft()

	if mv.curHMState.Connected && mv.doQueryParamsOnceConnected != nil {
		mv.doQuery(*mv.doQueryParamsOnceConnected)
		mv.doQueryParamsOnceConnected = nil
	}
}

func (mv *MainView) makeOverlayVisible() {
	mv.overlayMsgViewIsMinimized = false
	mv.overlayMsgView = mv.showMessagebox(
		"overlay_msg", "", "", &MessageboxParams{
			Buttons: []string{"Hide", "Reconnect & Retry", "Disconnect & Cancel"},
			OnButtonPressed: func(label string, idx int) {
				switch label {
				case "Hide":
					mv.hideOverlayMsgBox()
					mv.overlayMsgViewIsMinimized = true
					mv.printOverlayMsgInCmdline(mv.overlayText)
				case "Reconnect & Retry":
					mv.reconnect(true)
				case "Disconnect & Cancel":
					mv.disconnect()
				}
			},
			OnEsc: func() {
				mv.hideOverlayMsgBox()
				mv.overlayMsgViewIsMinimized = true
				mv.printOverlayMsgInCmdline(mv.overlayText)
			},
			NoFocus: false,
			Width:   70,
			Height:  8,

			BackgroundColor: tcell.ColorDarkBlue,
		},
	)
}

func (mv *MainView) printOverlayMsgInCmdline(overlayMsg string) {
	mv.printMsg(clearTviewFormatting(strings.Replace(overlayMsg, "\n", "", -1)), nlMsgLevelInfo)
}

func (mv *MainView) hideOverlayMsgBox() {
	// TODO: using pageNameMessage here directly is too hacky
	mv.hideModal(pageNameMessage+"overlay_msg", true)
}

func getStatuslineNumStr(icon string, num int, color string) string {
	mod := "-"
	if num > 0 {
		mod = "b"
	}

	return fmt.Sprintf("[%s:-:%s]%s %.2d[-:-:-]", color, mod, icon, num)
}

func (mv *MainView) updateTableHeader(msgs []core.LogMsg) (colNames []string) {
	// - maybe: Iterate all messages, and remove non-existing whitelisted fields
	// - If IncludeAll is set: build a list of tags which are not specified explicitly,
	//   and sort them

	existingTags := map[string]struct{}{
		FieldNameTime:    {},
		FieldNameMessage: {},
	}
	for _, msg := range msgs {
		for name := range msg.Context {
			existingTags[name] = struct{}{}
		}
	}

	numSticky := 0
	fields := make([]SelectQueryField, 0, len(mv.selectQuery.Fields))
	for _, fld := range mv.selectQuery.Fields {
		fields = append(fields, fld)

		if fld.Sticky {
			numSticky++
		}
	}

	// Move sticky ones to the front
	sort.SliceStable(fields, func(i, j int) bool {
		vi := 1
		if fields[i].Sticky {
			vi = 0
		}

		vj := 1
		if fields[j].Sticky {
			vj = 0
		}

		return vi < vj
	})

	explicit := make(map[string]struct{}, len(fields))
	for _, v := range fields {
		explicit[v.Name] = struct{}{}
	}

	if mv.selectQuery.IncludeAll {
		var implicitFields []SelectQueryField
		for v := range existingTags {
			if _, ok := explicit[v]; ok {
				continue
			}

			implicitFields = append(implicitFields, SelectQueryField{
				Name:        v,
				DisplayName: v,
			})
		}

		sort.Slice(implicitFields, func(i, j int) bool {
			return implicitFields[i].Name < implicitFields[j].Name
		})

		fields = append(fields, implicitFields...)
	}

	colNames = make([]string, 0, len(fields))
	for i, fld := range fields {
		displayName := fld.DisplayName

		// Special case for the time column. Pretty dirty, but will do for now.
		if fld.Name == "time" {
			tz := mv.params.Options.GetTimezone()
			zone, offset := time.Now().In(tz).Zone()
			if zone != "UTC" {
				sign := "+"
				if offset < 0 {
					sign = "-"
					offset = -offset
				}
				zone = fmt.Sprintf("UTC%s%02d", sign, offset/3600)
			}
			displayName = fmt.Sprintf("time (%s)", zone)
		}

		cell := newTableCellHeader(displayName)
		if _, ok := existingTags[fld.Name]; !ok {
			cell.SetTextColor(tcell.ColorRed)
		}
		if _, ok := explicit[fld.Name]; !ok {
			cell.SetTextColor(tcell.ColorLightGray)
		}

		mv.logsTable.SetCell(0, i, cell)

		colNames = append(colNames, fld.Name)
	}

	mv.logsTable.SetFixed(1, numSticky)

	return colNames
}

func (mv *MainView) applyLogs(resp *core.LogRespTotal) {
	mv.curLogResp = resp

	oldNumRows := mv.logsTable.GetRowCount()
	selectedRow, _ := mv.logsTable.GetSelection()
	offsetRow, offsetCol := mv.logsTable.GetOffset()

	mv.formatLogs()

	if !resp.LoadedEarlier {
		// Replaced all logs
		mv.logsTable.Select(len(resp.Logs)+1, 0)
		mv.logsTable.ScrollToEnd()
		mv.bumpTimeRange(true)
	} else {
		// Loaded more (earlier) logs
		numNewRows := mv.logsTable.GetRowCount() - oldNumRows
		mv.logsTable.SetOffset(offsetRow+numNewRows, offsetCol)
		mv.logsTable.Select(selectedRow+numNewRows, 0)
	}

	mv.printMsg(fmt.Sprintf("Query took: %s", resp.QueryDur.Round(1*time.Millisecond)), nlMsgLevelInfo)
}

func (mv *MainView) formatLogs() {
	resp := mv.curLogResp
	if resp == nil {
		resp = &core.LogRespTotal{}
	}

	histogramData := make(map[int]int, len(resp.MinuteStats))
	for k, v := range resp.MinuteStats {
		histogramData[int(k)] = v.NumMsgs
	}

	mv.histogram.SetData(histogramData)

	// TODO: perhaps optimize it, instead of clearing and repopulating whole table
	mv.logsTable.Clear()

	// Update existingTagNames
	mv.existingTagNames = map[string]struct{}{
		FieldNameTime:    {},
		FieldNameMessage: {},
	}
	for _, msg := range resp.Logs {
		for name := range msg.Context {
			mv.existingTagNames[name] = struct{}{}
		}
	}

	// Update table header
	colNames := mv.updateTableHeader(resp.Logs)

	mv.logsTable.SetCell(
		rowIdxLoadOlder, 0,
		newTableCellButton("< MOAR ! >"),
	)

	tz := mv.params.Options.GetTimezone()

	// Add all available logs
	for i, rowIdx := 0, 2; i < len(resp.Logs); i, rowIdx = i+1, rowIdx+1 {
		msg := resp.Logs[i]

		// TODO: make it configurable
		msgColor := tcell.ColorWhite
		levelName := "info"
		if ln, ok := msg.Context["level_name"]; ok {
			levelName = ln
		} else if strings.Contains(msg.Msg, "[D]") {
			levelName = "debug"
		} else if strings.Contains(msg.Msg, "[I]") {
			levelName = "info"
		} else if strings.Contains(msg.Msg, "[W]") {
			levelName = "warn"
		} else if strings.Contains(msg.Msg, "[E]") {
			levelName = "error"
		}

		switch levelName {
		case "debug":
			msgColor = tcell.ColorLightGreen
		case "info":
			// Same as default, but just to mention it explicitly
			msgColor = tcell.ColorWhite
		case "warn":
			msgColor = tcell.ColorYellow
		case "error":
			msgColor = tcell.ColorPink
		}

		timeStr := msg.Time.In(tz).Format(logsTableTimeLayout)
		if msg.DecreasedTimestamp {
			timeStr = ""
		}

		for i, colName := range colNames {
			var cell *tview.TableCell

			switch colName {
			case FieldNameTime:
				cell = newTableCellLogmsg(timeStr).SetTextColor(tcell.ColorLightBlue)
			case FieldNameMessage:
				cell = newTableCellLogmsg(tview.Escape(msg.Msg)).SetTextColor(msgColor)
			default:
				cell = newTableCellLogmsg(msg.Context[colName]).SetTextColor(msgColor)
			}

			mv.logsTable.SetCell(rowIdx, i, cell)
		}

		mv.logsTable.GetCell(rowIdx, 0).SetReference(msg)
	}

	mv.bumpStatusLineRight()
}

func (mv *MainView) bumpStatusLineLeft() {
	sb := strings.Builder{}

	lsmanState := mv.curHMState
	if lsmanState == nil {
		// We haven't received a single HMState update, so just use the zero value.
		lsmanState = &core.LStreamsManagerState{}
	}

	if !lsmanState.Connected && !lsmanState.NoMatchingLStreams {
		sb.WriteString("conn ")
	} else if lsmanState.Busy {
		sb.WriteString("busy ")
	} else {
		sb.WriteString("idle ")
	}

	numIdle := len(lsmanState.LStreamsByState[core.LStreamClientStateConnectedIdle])
	numBusy := len(lsmanState.LStreamsByState[core.LStreamClientStateConnectedBusy])
	numOther := lsmanState.NumLStreams - numIdle - numBusy

	sb.WriteString(getStatuslineNumStr("🖳", numIdle, "green"))
	sb.WriteString(" ")
	sb.WriteString(getStatuslineNumStr("🖳", numBusy, "orange"))
	sb.WriteString(" ")
	sb.WriteString(getStatuslineNumStr("🖳", numOther, "red"))

	sb.WriteString(" | ")
	sb.WriteString(mv.lstreamsSpec)

	mv.statusLineLeft.SetText(sb.String())
}

func (mv *MainView) bumpStatusLineRight() {
	selectedRow, _ := mv.logsTable.GetSelection()
	selectedRow -= 1

	var selectedRowStr string
	if selectedRow >= 1 {
		selectedRowStr = strconv.Itoa(selectedRow)
	} else {
		selectedRowStr = "-"
	}

	if mv.curLogResp != nil {
		mv.statusLineRight.SetText(fmt.Sprintf(
			"%s / %d / %d",
			selectedRowStr, len(mv.curLogResp.Logs), mv.curLogResp.NumMsgsTotal,
		))
	} else {
		mv.statusLineRight.SetText("-")
	}
}

func newTableCellHeader(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorLightBlue).
		SetAttributes(tcell.AttrBold).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
}

func newTableCellLogmsg(text string) *tview.TableCell {
	return tview.NewTableCell(text).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignLeft)
}

func newTableCellButton(text string) *tview.TableCell {
	return tview.NewTableCell(text).SetTextColor(tcell.ColorWhite).SetAlign(tview.AlignCenter)
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

func (mv *MainView) setQuery(q string) {
	if mv.queryInput.GetText() != q {
		mv.queryInput.SetText(q)
	}
	mv.query = q
}

func (mv *MainView) setSelectQuery(sqp *SelectQueryParsed) {
	mv.selectQuery = sqp
}

func (mv *MainView) setTimeRange(from, to TimeOrDur) {
	if from.IsZero() {
		// TODO: maybe better error handling
		panic("from can't be zero")
	}

	mv.from = from
	mv.to = to

	mv.formatTimeRange()
}

func (mv *MainView) formatTimeRange() {
	tz := mv.params.Options.GetTimezone()

	mv.bumpTimeRange(false)

	rangeDur := mv.actualTo.Sub(mv.actualFrom)

	var timeStr string
	if !mv.to.IsZero() {
		timeStr = fmt.Sprintf("%s to %s (%s)", mv.from.In(tz).Format(inputTimeLayout), mv.to.In(tz).Format(inputTimeLayout), formatDuration(rangeDur))
	} else if mv.from.IsAbsolute() {
		timeStr = fmt.Sprintf("%s to now (%s)", mv.from.In(tz).Format(inputTimeLayout), formatDuration(rangeDur))
	} else {
		timeStr = fmt.Sprintf("last %s", TimeOrDur{Dur: -mv.from.Dur})
	}

	mv.timeLabel.SetText(timeStr)
	mv.topFlex.ResizeItem(mv.timeLabel, len(timeStr), 0)
}

// bumpTimeRange only does something useful if the time is relative to current time.
func (mv *MainView) bumpTimeRange(updateHistogramRange bool) {
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
		mv.actualToForQuery = mv.actualTo
	} else {
		mv.actualTo = time.Now()
		mv.actualToForQuery = time.Time{}
	}

	// Snap both actualFrom and actualTo to the 1m grid, rounding forward.
	mv.actualFrom = truncateCeil(mv.actualFrom, 1*time.Minute)
	mv.actualTo = truncateCeil(mv.actualTo, 1*time.Minute)
	if !mv.actualToForQuery.IsZero() {
		mv.actualToForQuery = truncateCeil(mv.actualToForQuery, 1*time.Minute)
	}

	// If from is after than to, swap them.
	if mv.actualFrom.After(mv.actualTo) {
		mv.actualFrom, mv.actualTo = mv.actualTo, mv.actualFrom

		// Sanity check: mv.actualToForQuery can never be zero in this case.
		if mv.actualToForQuery.IsZero() {
			panic("actualToForQuery is zero while actualFrom is after actualTo")
		}
	}

	// Also update the histogram
	if updateHistogramRange {
		mv.histogram.SetRange(int(mv.actualFrom.Unix()), int(mv.actualTo.Unix()))
	}
}

func truncateCeil(t time.Time, dur time.Duration) time.Time {
	t2 := t.Truncate(dur)
	if t2.Equal(t) {
		return t
	}

	return t2.Add(dur)
}

func (mv *MainView) SetTimeRange(from, to TimeOrDur) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.setTimeRange(from, to)
	})
}

func (mv *MainView) setLStreams(s string) {
	mv.lstreamsSpec = s
}

type doQueryParams struct {
	// If dontAddHistoryItem is true, the browser-like history will not be
	// populated with a new item (it should be used exactly when we're navigating
	// this browser-like history back and forth)
	dontAddHistoryItem bool
}

func (mv *MainView) doQuery(params doQueryParams) {
	mv.params.OnLogQuery(core.QueryLogsParams{
		From:  mv.actualFrom,
		To:    mv.actualToForQuery,
		Query: mv.query,

		DontAddHistoryItem: params.dontAddHistoryItem,
	})
}

func (mv *MainView) DoQuery(dqp doQueryParams) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.doQuery(dqp)
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
	OnEsc           func()

	Width, Height int

	NoFocus bool

	BackgroundColor tcell.Color
}

func (mv *MainView) showMessagebox(
	msgID, title, message string, params *MessageboxParams,
) *MessageView {
	var msgv *MessageView

	if params == nil {
		params = &MessageboxParams{}
	}

	if params.Buttons == nil {
		params.Buttons = []string{"OK"}
	}

	if params.OnButtonPressed == nil {
		params.OnButtonPressed = func(label string, idx int) {
			msgv.Hide()
		}
	}

	if params.OnEsc == nil {
		params.OnEsc = func() {
			msgv.Hide()
		}
	}

	msgv = NewMessageView(mv, &MessageViewParams{
		App: mv.params.App,

		MessageID:       msgID,
		Title:           title,
		Message:         message,
		Buttons:         params.Buttons,
		OnButtonPressed: params.OnButtonPressed,
		OnEsc:           params.OnEsc,

		Width:  params.Width,
		Height: params.Height,

		NoFocus: params.NoFocus,

		BackgroundColor: params.BackgroundColor,
	})
	msgv.Show()

	return msgv
}

func (mv *MainView) ShowMessagebox(
	msgID, title, message string, params *MessageboxParams,
) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.showMessagebox(msgID, title, message, params)
	})
}

func (mv *MainView) HideMessagebox(msgID string, popFocusStack bool) {
	mv.params.App.QueueUpdateDraw(func() {
		mv.hideModal(pageNameMessage+msgID, popFocusStack)
	})
}

func (mv *MainView) showOriginalMsg(msg core.LogMsg) {
	lnOffsetUp := 1000   // How many surrounding lines to show, up
	lnOffsetDown := 1000 // How many surrounding lines to show, down
	lnBegin := msg.LogLinenumber - lnOffsetUp
	if lnBegin <= 0 {
		lnOffsetUp += lnBegin - 1
		lnBegin = 1
	}

	s := fmt.Sprintf(
		"ssh -t %s 'vim +\"set ft=messages\" +%d <(tail -n +%d %s | head -n %d)'\n\n%s",
		msg.Context["source"], lnOffsetUp+1, lnBegin, msg.LogFilename, lnOffsetUp+lnOffsetDown,
		tview.Escape(msg.OrigLine),
	)

	mv.showMessagebox("msg", "Message", s, &MessageboxParams{
		Width:  120,
		Height: 20,
	})
}

func (mv *MainView) showModal(name string, primitive tview.Primitive, width, height int, focus bool) {
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

	if focus {
		mv.params.App.SetFocus(primitive)
	} else {
		mv.popFocusStack()
	}
}

func (mv *MainView) hideModal(name string, popFocusStack bool) {
	prevFocused := mv.params.App.GetFocus()

	mv.rootPages.RemovePage(name)
	if popFocusStack {
		mv.popFocusStack()
	} else {
		// Feels hacky, but I didn't find another way: apparently adding/removing
		// pages inevitably messes with focus, and so if we want to keep it
		// unchanged, we have to set it back manually.
		mv.params.App.SetFocus(prevFocused)
	}
}

func (mv *MainView) popFocusStack() {
	l := len(mv.modalsFocusStack)
	mv.params.App.SetFocus(mv.modalsFocusStack[l-1])
	mv.modalsFocusStack = mv.modalsFocusStack[:l-1]
}

func (mv *MainView) getQueryFull() QueryFull {
	ftr := FromToRange{mv.from, mv.to}
	return QueryFull{
		Time:        ftr.String(),
		Query:       mv.query,
		LStreams:    mv.lstreamsSpec,
		SelectQuery: mv.selectQuery.Marshal(),
	}
}

func (mv *MainView) setFocus(p tview.Primitive) {
	mv.params.App.SetFocus(p)
}

// queueUpdateLater is a hackish helper, it's useful when we are IN the UI
// event loop, and we want to queue another update to the event loop which will
// fire after the current handler is done.
//
// We sometimes want this due to hackery with focusing widgets: e.g. if we're
// handling the update from DropDown when its list is open, so the current
// focus is this internal dropdown's list. If we try to show the messagebox
// right from the handler which will later also hide the list, then the
// messagebox will remember that when it's closed it needs to set the focus to
// the list, and when that finally happens, the list isn't visible anymore, so
// we're in a focus trap.
//
// If we use this hack with queueUpdateLater, then it's guaranteed to be called
// only after the current handler (for the dropdown) is done already and the
// focus is removed from the internal list, so the messagebox correctly remembers
// to focus the dropdown itself.
func (mv *MainView) queueUpdateLater(f func()) {
	go func() {
		mv.params.App.QueueUpdateDraw(f)
	}()
}

func (mv *MainView) openQueryEditView() {
	mv.params.QueryHistory.Load()
	mv.params.QueryHistory.Reset()
	mv.queryEditView.Show(mv.getQueryFull())
}

func (mv *MainView) disconnect() {
	mv.curLogResp = nil
	mv.sendLStreamsChangeOnNextQuery = true
	mv.params.OnDisconnectRequest()
}

// handleQueryError shows the right messagebox based on the error cause.
func (mv *MainView) handleQueryError(err error) {
	if errors.Cause(err) == core.ErrBusyWithAnotherQuery ||
		errors.Cause(err) == core.ErrNotYetConnected {
		// In this particular error ("busy with another query"), show a dialog
		// with the additional button "Details", which can be used to open the
		// details of the query in progress.

		msgID := "busyWithAnotherQuery"

		mv.showMessagebox(
			msgID,
			"Log query error",
			err.Error(),
			&MessageboxParams{
				Buttons: []string{"OK", "Details"},
				OnButtonPressed: func(label string, idx int) {
					// Whatever button the user pressed, we hide the dialog. Keep in mind
					// it needs to happen _before_ we call makeOverlayVisible() below.

					// TODO: using pageNameMessage here directly is too hacky
					mv.hideModal(pageNameMessage+msgID, true)

					switch label {
					case "OK":
						// Nothing special to do, just hide the dialog (which was already done above)

					case "Details":
						if mv.overlayMsgView != nil && mv.overlayMsgViewIsMinimized {
							mv.makeOverlayVisible()
							mv.bumpOverlay()
						}

						// TODO: would be useful to implement force-override right from this dialog,
						// but keep in mind it should work both when we just send a new query,
						// and when the "full" query changes (with lstreams, timeline etc).
						// It's non-essential though, so for now I ignore it.

						//case "Override":
						//// TODO: using pageNameMessage here directly is too hacky
						//mv.hideModal(pageNameMessage+msgID, true)
						//mv.reconnect(true)
					}
				},
				Width:  40,
				Height: 8,

				BackgroundColor: tcell.ColorDarkRed,
			},
		)
	} else {
		// In all other errors, open a regular dialog.
		mv.showMessagebox("err", "Log query error", err.Error(), &MessageboxParams{
			BackgroundColor: tcell.ColorDarkRed,
		})
	}
}

// reconnect initiates reconnection to all the log streams. If repeatQuery
// is true, then after reconnecting, the current query will be repeated, too.
func (mv *MainView) reconnect(repeatQuery bool) {
	mv.sendLStreamsChangeOnNextQuery = false

	if repeatQuery {
		mv.doQueryParamsOnceConnected = &doQueryParams{}
	} else {
		mv.doQueryParamsOnceConnected = nil
	}

	mv.params.OnReconnectRequest()
}
