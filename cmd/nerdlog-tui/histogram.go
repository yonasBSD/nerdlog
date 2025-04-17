package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

/*
const (
	qblock0000 = ' '
	qblock0001 = '▗'
	qblock0010 = '▖'
	qblock0011 = '▄'
	qblock0100 = '▝'
	qblock0101 = '▐'
	qblock0110 = '▞'
	qblock0111 = '▟'
	qblock1000 = '▘'
	qblock1001 = '▚'
	qblock1010 = '▌'
	qblock1011 = '▙'
	qblock1100 = '▀'
	qblock1101 = '▜'
	qblock1110 = '▛'
	qblock1111 = '█'
)
*/

var (
	qblocks = []rune{
		' ', '▗', '▖', '▄', '▝', '▐', '▞', '▟',
		'▘', '▚', '▌', '▙', '▀', '▜', '▛', '█',
	}
)

type Histogram struct {
	*tview.Box

	// from and to specify the range, they're never zero.
	from int
	to   int

	binSize int

	// data is a map from the value in beginning of a bin to the size of that
	// bin.
	data map[int]int

	// getXMarks returns where to put marks on X axis
	getXMarks func(from, to int, numChars int) []int

	// xFormat formats values for X axis
	xFormat func(v int) string

	// formatCursor formats values for cursor
	formatCursor func(from int, to *int, width int) string

	// TODO explain
	snapDataBinsInChartDot func(dataBinsInChartBar int) int

	// selected is a handler which is called when the user has finished selecting
	// a range. The from is inclusive, the to is not.
	selected func(from, to int)

	// curMarks is returned from the last call to getXMarks
	curMarks []int

	// cursor is the current cursor position in seconds
	// (same unit as from, to, etc). It must be divisible by the block size: if the
	// block size is 120 (two minutes), it means cursor must be divisible by 120.
	// It also means that resizing the window can change the cursor.
	cursor int
	// selectionStart, if not 0, is the position in seconds where selection has
	// started. It can be either larger or smaller than cursor. If 0, it means
	// there is no selection in progress.
	selectionStart int

	fldData *fieldData

	externalCursor int
}

func NewHistogram() *Histogram {
	return &Histogram{
		Box: tview.NewBox(),
		// TODO: set default getXMarks and xFormat, we just don't have defaults yet.
	}
}

func (h *Histogram) SetRange(from, to int) *Histogram {
	h.from = from
	h.to = to

	// Reset cursor and selection too
	h.cursor = h.to - h.binSize*h.getDataBinsInChartBar()
	h.cursor = h.alignCursor(h.cursor, false)
	h.selectionStart = 0

	return h
}

func (h *Histogram) alignCursor(cursor int, isCeiling bool) int {
	divisor := h.getDataBinsInChartBar() * h.binSize
	cursor -= h.from
	remainder := cursor % divisor
	cursor -= remainder
	if isCeiling && remainder > 0 {
		cursor += divisor
	}
	cursor += h.from

	return cursor
}

func (h *Histogram) SetBinSize(binSize int) *Histogram {
	h.binSize = binSize

	return h
}

func (h *Histogram) SetData(data map[int]int) *Histogram {
	h.data = data

	return h
}

func (h *Histogram) SetXFormatter(xFormat func(v int) string) *Histogram {
	h.xFormat = xFormat

	return h
}

func (h *Histogram) SetCursorFormatter(formatCursor func(from int, to *int, width int) string) *Histogram {
	h.formatCursor = formatCursor

	return h
}

func (h *Histogram) SetDataBinsSnapper(snapDataBinsInChartDot func(dataBinsInChartBar int) int) *Histogram {
	h.snapDataBinsInChartDot = snapDataBinsInChartDot

	return h
}

func (h *Histogram) SetXMarker(getXMarks func(from, to int, numChars int) []int) *Histogram {
	h.getXMarks = getXMarks

	return h
}

func (h *Histogram) SetExternalCursor(externalCursor int) *Histogram {
	h.externalCursor = externalCursor

	return h
}

func (h *Histogram) Draw(screen tcell.Screen) {
	h.Box.DrawForSubclass(screen, h)
	x, y, width, height := h.GetInnerRect()

	fldMarginLeft := 0

	// We multiply width and height by 2 because we use quadrant graphics,
	// so one character is a 2x2 field.
	fldWidth := (width - fldMarginLeft) * 2
	fldHeight := (height - 1) * 2 // One line for axis

	fldData := h.genFieldData(fldWidth, fldHeight)
	if fldData == nil {
		// Field is too small
		// TODO: clean whole area
		return
	}

	h.fldData = fldData
	h.cursor = h.alignCursor(h.cursor, false)

	fldMarginLeft = (width - fldData.effectiveWidthRunes) / 2

	lines := h.fldDataToLines(fldData.dots)

	for lineY, line := range lines {
		tview.Print(screen, line, x+fldMarginLeft, y+lineY, width-fldMarginLeft, tview.AlignLeft, tcell.ColorLightGray)
	}

	// Print max label in the top left corner
	maxLabel := fmt.Sprintf("%d", fldData.yScale)
	maxLabelOffset := fldMarginLeft - len(maxLabel) - 1
	printDot := true
	if maxLabelOffset < 0 {
		maxLabelOffset = 0
		printDot = false // no space for it, better to omit it
	}
	if printDot {
		maxLabel = maxLabel + "[yellow]▀[-]"
	}
	tview.Print(screen, maxLabel, x+maxLabelOffset, y, width-maxLabelOffset, tview.AlignLeft, tcell.ColorWhite)

	h.curMarks = h.getXMarks(h.from, h.to, width-fldMarginLeft)

	sb := strings.Builder{}
	numRunes := 0

	for _, mark := range h.curMarks {
		markStr := h.xFormat(mark)
		dotCoord := h.valToCoord(mark)

		charCoord := dotCoord / 2
		remaining := charCoord - numRunes
		for i := 0; i < remaining; i++ {
			sb.WriteRune(' ')
			numRunes++
		}

		sb.WriteString("[yellow]")
		if (dotCoord & 0x01) != 0 {
			sb.WriteRune('▝')
		} else {
			sb.WriteRune('▘')
		}
		sb.WriteString("[-] ")
		numRunes += 2
		//sb.WriteString("^ ")
		sb.WriteString(markStr)
		numRunes += len(clearTviewFormatting(markStr))
	}

	tview.Print(screen, sb.String(), x+fldMarginLeft, y+height-1, width-fldMarginLeft, tview.AlignLeft, tcell.ColorWhite)

	// If we're in the focus, then also draw the cursor and maybe selection marks.
	if h.HasFocus() {
		selScaleLines := h.fldDataToLines(fldData.selScaleDots)
		// There should be exactly one line
		line := selScaleLines[0]
		lineLen := len(fldData.selScaleDots[0]) / 2

		var selMark string
		if !h.IsSelectionActive() {
			selMark = fmt.Sprintf("[%s]", h.formatCursor(h.cursor, nil, h.binSize))
		} else {
			selStart, selEnd := h.GetSelection()
			selMark = fmt.Sprintf("[%s]", h.formatCursor(selStart, &selEnd, h.binSize))
		}

		leftOffset := fldMarginLeft + fldData.selScaleOffset/2

		tview.Print(screen, line, x+leftOffset, y+height-1, width-leftOffset, tview.AlignLeft, tcell.ColorGreen)

		// Print the selection range description
		selMarkOffset := leftOffset + lineLen + 1
		freeSpaceRight := width - (selMarkOffset + len(selMark))
		if freeSpaceRight < 0 {
			// The selMark text doesn't fit, so we move it to the left so it's on the
			// right edge.
			selMarkOffset += freeSpaceRight
		} else {
			// The selMark text fits, but we intentionally have a spacing of 1 char
			// before it, and to avoid having some other stuff from underneath
			// showing up in that spacing, we fill it out with a " ".
			selMarkOffset--
			selMark = " " + selMark
		}
		tview.Print(screen, selMark, x+selMarkOffset, y+height-1, width-selMarkOffset, tview.AlignLeft, tcell.ColorGreen)

		// Also print the bar value (or sum of all selected values, if selection is active)
		var valToPrint string
		if !h.IsSelectionActive() {
			valToPrint = fmt.Sprintf("(%d)", fldData.cursorVal)
		} else {
			valToPrint = fmt.Sprintf("(total %d)", fldData.selectedValsSum)
		}

		totalMarkOffset := x + leftOffset + lineLen + 1
		freeSpaceRight = width - (totalMarkOffset + len(valToPrint))
		if freeSpaceRight < 0 {
			// The valToPrint text doesn't fit, so we move it to the left so it's on
			// the right edge.
			totalMarkOffset += freeSpaceRight
		}
		tview.Print(screen, valToPrint, totalMarkOffset, y, width-totalMarkOffset, tview.AlignLeft, tcell.ColorGreen)
	}

	// Draw a pointer to the external cursor.
	// TODO: implement in a better way.
	extCursorCoord := h.valToCoord(h.externalCursor)
	extCursorOffset := extCursorCoord / 2
	tview.Print(screen, "^", x+fldMarginLeft+extCursorOffset, y+height-1, width, tview.AlignLeft, tcell.ColorRed)
}

func (h *Histogram) getDataBinsInChartBar() int {
	if h.fldData == nil {
		return 1
	}

	return h.fldData.dataBinsInChartBar
}

func (h *Histogram) getChartBarWidth() int {
	if h.fldData == nil {
		return 1
	}

	return h.fldData.chartBarWidth
}

func (h *Histogram) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return h.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		maxCursor := h.alignCursor(h.to-h.binSize*h.getDataBinsInChartBar(), true)

		moveLeft := func() {
			h.cursor -= h.binSize * h.getDataBinsInChartBar()
			if h.cursor < h.from {
				h.cursor = h.from
			}
		}

		moveRight := func() {
			h.cursor += h.binSize * h.getDataBinsInChartBar()
			if h.cursor > maxCursor {
				h.cursor = maxCursor
			}
		}

		moveLeftLong := func() {
			target := h.from
			for _, mark := range h.curMarks {
				if mark >= h.cursor {
					break
				}

				target = mark
			}

			h.cursor = h.alignCursor(target, false)
		}

		moveRightLong := func() {
			// Since the cursor can "encloses" the mark, we need to move it right once,
			// so that if the cursor was on the mark, it won't be on that mark anymore.
			moveRight()

			for _, mark := range h.curMarks {
				if mark > maxCursor {
					break
				}

				if mark >= h.cursor {
					h.cursor = h.alignCursor(mark, false)
					return
				}
			}

			h.cursor = maxCursor
		}

		moveBeginning := func() {
			h.cursor = h.from
		}

		moveEnd := func() {
			h.cursor = maxCursor
		}

		selectionEnd := func() {
			h.selectionStart = 0
		}

		selectionToggle := func() {
			if h.selectionStart == 0 {
				// Start selection
				h.selectionStart = h.cursor
			} else {
				// End selection
				selectionEnd()
			}
		}

		selectionApplyIfActive := func() {
			if h.selectionStart != 0 && h.selected != nil {
				from, to := h.GetSelection()
				h.selected(from, to)
			}
		}

		switch key := event.Key(); key {
		case tcell.KeyRune: // Regular character.
			if event.Modifiers()&tcell.ModAlt > 0 {
				switch event.Rune() {
				case 'b':
					moveLeftLong()
				case 'f':
					moveRightLong()
				}
			} else {
				switch event.Rune() {
				case 'h':
					moveLeft()
				case 'l':
					moveRight()
				case 'b':
					moveLeftLong()
				case 'w', 'e':
					moveRightLong()
				case 'g', '^':
					moveBeginning()
				case 'G', '$':
					moveEnd()
				case 'v', ' ':
					selectionApplyIfActive()
					selectionToggle()
				case 'q':
					selectionEnd()
				case 'o':
					if h.selectionStart > 0 {
						h.cursor, h.selectionStart = h.selectionStart, h.cursor
					}
				}
			}

		case tcell.KeyLeft:
			moveLeft()
		case tcell.KeyRight:
			moveRight()

		case tcell.KeyPgUp:
			moveLeftLong()
		case tcell.KeyPgDn:
			moveRightLong()

		case tcell.KeyHome, tcell.KeyCtrlA:
			moveBeginning()
		case tcell.KeyEnd, tcell.KeyCtrlE:
			moveEnd()

		case tcell.KeyEnter:
			selectionApplyIfActive()
			selectionToggle()

		case tcell.KeyEsc:
			selectionEnd()
		}
	})
}

// GetSelection, if selection is active, returns it, from being inclusive and
// to being exclusive. The "direction" of the selection doesn't matter: from will
// never be larger than to.
//
// If no selection is active, returns (0, 0).
func (h *Histogram) GetSelection() (selStart, selEnd int) {
	if h.selectionStart == 0 {
		return 0, 0
	}

	selStart = h.selectionStart
	selEnd = h.cursor
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}

	selEnd += h.binSize * h.getDataBinsInChartBar()

	return selStart, selEnd
}

func (h *Histogram) SetSelectedFunc(handler func(from, to int)) *Histogram {
	h.selected = handler
	return h
}

func (h *Histogram) IsSelectionActive() bool {
	return h.selectionStart != 0
}

type fieldData struct {
	dots [][]bool

	dataBinsInChartBar int
	chartBarWidth      int

	// dotYScale is how many messages in one dot
	dotYScale int

	// max is the actual (unrounded) max value in the chart bar (not in the data,
	// but in the chart bar, which might be composed of more than one data bin)
	max int

	// yScale is the maximum value as per chart (it's larger than max).
	yScale int

	effectiveWidthDots  int
	effectiveWidthRunes int

	// selScaleDots is another small field (height 2, width depends on the size
	// of the selection) representing the selection scale below. It's only actually
	// drawn if we're in focus.
	selScaleDots [][]bool
	// selScaleOffset is the X offset of selScaleDots, from the left side.
	selScaleOffset int

	// cursorVal is the value of the bar currently selected by the cursor
	cursorVal int
	// selectedValsSum is the sum of all bars selected currently (if selection is
	// in progress)
	selectedValsSum int
}

// genFieldData returns a 2-dimensional field as nested slices: [y][x].
// If the field is too small, returns nil.
func (h *Histogram) genFieldData(width, height int) *fieldData {
	foc := h.HasFocus()

	scale := getOptimalScale(h.from, h.to, h.binSize, width, h.snapDataBinsInChartDot)
	if scale == nil {
		return nil
	}

	h.from = scale.from
	h.to = scale.to
	numDataBins := scale.numDataBins
	dataBinsInChartBar := scale.dataBinsInChartBar
	chartBarWidth := scale.chartBarWidth

	valAt := func(idx, n int) int {
		var val int
		for i := 0; i < n; i++ {
			val += h.data[h.from+(idx+i)*h.binSize]
		}
		return val
	}

	isCursorAt := func(idx, n int) bool {
		for i := 0; i < n; i++ {
			if h.cursor == h.from+(idx+i)*h.binSize {
				return true
			}
		}

		return false
	}

	selStart, selEnd := h.GetSelection()
	if selStart == 0 || selEnd == 0 {
		selStart = h.cursor
		selEnd = h.cursor + dataBinsInChartBar*h.binSize
	}

	isSelectedAt := func(idx, n int) bool {
		for i := 0; i < n; i++ {
			v := h.from + (idx+i)*h.binSize
			if v >= selStart && v < selEnd {
				return true
			}
		}

		return false
	}

	//effectiveWidthDots := numDataBins

	// Find the max bin value per bar on the chart
	max := 0
	for xData := 0; xData < numDataBins; xData = xData + dataBinsInChartBar {
		val := valAt(xData, dataBinsInChartBar)

		if max < val {
			max = val
		}
	}

	dotYScale := (max + height - 1) / height
	// TODO: round it

	// Allocate all the slices so we have the field ready
	dots := make([][]bool, height)
	for y := 0; y < height; y++ {
		dots[y] = make([]bool, width)
	}

	selScaleDots := make([][]bool, 2)
	for y := 0; y < 2; y++ {
		selScaleDots[y] = make([]bool, width)
	}

	selOffsetStart := -1 // The coord of selection start
	selOffsetEnd := -1   // The coord of selection end
	offsetLast := -1     // The last effective chart coord

	cursorVal := 0
	selectedValsSum := 0

	// Iterate over data and set dots to true
	for xData, xChart := 0, 0; xData < numDataBins; xData, xChart = xData+dataBinsInChartBar, xChart+chartBarWidth {
		val := valAt(xData, dataBinsInChartBar)
		sel := isSelectedAt(xData, dataBinsInChartBar)
		crs := isCursorAt(xData, dataBinsInChartBar)

		if crs {
			cursorVal = val
		}

		if sel {
			selectedValsSum += val
		}

		for y := 0; y < height; y++ {
			on := val > y*dotYScale

			// As an optimization: if the dot is off and the cursor is not here, it
			// means that all other dots in this column will be off, so we're done
			// with this column.
			if !on && !(foc && sel) {
				break
			}

			// If cursor is here, we need to inverse the dot.
			if foc && sel {
				on = !on
			}

			// If the dots are on, set them to true.
			if on {
				for i := 0; i < chartBarWidth; i++ {
					dots[height-y-1][xChart+i] = true
				}
			}
		}

		for i := 0; i < chartBarWidth; i++ {
			offsetLast = (xChart + i)
			if (offsetLast & 0x01) != 0 {
				offsetLast += 1
			}

			if sel {
				// When selection starts, remember its offset
				// (and also make sure it's even)
				if selOffsetStart == -1 {
					selOffsetStart = (xChart + i)
					if (selOffsetStart & 0x01) != 0 {
						selOffsetStart -= 1
					}
				}
				selScaleDots[0][xChart+i] = true
			} else if selOffsetStart != -1 && selOffsetEnd == -1 {
				selOffsetEnd = offsetLast
			}

			if crs {
				selScaleDots[1][xChart+i] = true
			}
		}
	}

	// Cut the empty data from selScaleDots
	if selOffsetEnd == -1 {
		selOffsetEnd = offsetLast
	}
	for i := range selScaleDots {
		if selOffsetEnd != -1 {
			selScaleDots[i] = selScaleDots[i][:selOffsetEnd]
		}
		selScaleDots[i] = selScaleDots[i][selOffsetStart:]
	}

	effectiveWidthDots := numDataBins / dataBinsInChartBar * chartBarWidth
	effectiveWidthRunes := effectiveWidthDots / 2
	if (effectiveWidthDots & 0x01) > 0 {
		effectiveWidthRunes++
	}

	return &fieldData{
		dots:               dots,
		dataBinsInChartBar: dataBinsInChartBar,
		chartBarWidth:      chartBarWidth,

		max:       max,
		dotYScale: dotYScale,
		yScale:    dotYScale * height,

		effectiveWidthDots:  effectiveWidthDots,
		effectiveWidthRunes: effectiveWidthRunes,

		selScaleDots:   selScaleDots,
		selScaleOffset: selOffsetStart,

		cursorVal:       cursorVal,
		selectedValsSum: selectedValsSum,
	}

	/*
		ret := [][]bool{}

		for y := 0; y < height; y++ {
			row := []bool{}
			for x := 0; x < width; x++ {
				row = append(row, rand.Intn(10) < 5)
			}

			ret = append(ret, row)
		}

		return ret
	*/

	//return [][]bool{
	//[]bool{false, false, false, false, true, false, false, true, true, false, true, false, false, true, true, false, true, false, false, true, true, false, true, false},
	//[]bool{false, true, true, false, true, false, false, true, false, false, true, false, false, false, true, false, false, false, false, true, true, false, false, false},
	//[]bool{false, true, true, false, true, false, false, true, false, false, true, false, false, true, true, false, false, false, false, true, true, false, false, false},
	//[]bool{false, true, true, false, false, false, false, true, false, false, true, false, false, true, true, false, true, false, false, true, true, false, true, false},
	//}
}

// histogramScale represents key parameters about drawing a histogram.
// See getOptimalScale.
type histogramScale struct {
	// From is the updated starting point, might be smaller than the original one
	// due to snapping. The client code must use this updated one.
	from int
	// To is the updated ending point, might be bigger than the original one due
	// to snapping. The client code must use this updated one.
	to int
	// numDataBins is the number of data bins that we need to include in the
	// histogram in total.
	numDataBins int
	// dataBinsInChartBar specifies how many data bins a single bar in the
	// histogram represents.
	dataBinsInChartBar int
	// chartBarWidth specifies how wide is a single chart bar, measured in chart
	// dots (where a single character on the screen has 2 dots).
	chartBarWidth int
}

// getOptimalScale tries to calculate optimal scale to draw a histogram.
// The from and to represent the total histogram range, and binSize specifies
// the max resolution for the histogram. For example, if the histogram
// is used as a timeline, and the from and to are seconds (as in Unix timestamp),
// and the max resolution the histogram must have is 1 minute, then binSize should
// be set to 60. This way, the histogram bar will always include at least 1
// minute (60 seconds) worth of data.
//
// The width is the total width of the histogram on screen, in pixels or chart
// dots or however you like to call them.
//
// Then, snapDataBinsInChartDot is a callback which takes some arbitrary number of
// data bins in the chart bar, and returns potentially larger number. E.g. again
// in case of timeline histogram, it would make sense to snap values such as 18 mins
// to 20 mins, to make the histogram easier to use.
//
// The calculated values are such that the histogram is as big on the screen as
// possible, and as as detailed as possible, given the constraints.
func getOptimalScale(
	from, to, binSize, width int,
	snapDataBinsInChartDot func(dataBinsInChartBar int) int,
) *histogramScale {
	if width <= 0 {
		return nil
	}

	numDataBins := (to - from) / binSize

	if numDataBins == 0 {
		return nil
	}

	dataBinsInChartBar := (numDataBins + width - 1) / width
	dataBinsInChartBar = snapDataBinsInChartDot(dataBinsInChartBar)

	divisor := dataBinsInChartBar * binSize

	fromRemainder := from % divisor
	if fromRemainder > 0 {
		from -= fromRemainder
	}

	toRemainder := to % divisor
	if toRemainder > 0 {
		to += (divisor - toRemainder)
	}

	// If we had to expand the range, recalculate numDataBins and
	// dataBinsInChartBar again.
	if fromRemainder > 0 || toRemainder > 0 {
		numDataBins = (to - from) / binSize
		dataBinsInChartBar = (numDataBins + width - 1) / width
		dataBinsInChartBar = snapDataBinsInChartDot(dataBinsInChartBar)
	}

	numBars := numDataBins / dataBinsInChartBar
	if numBars == 0 {
		return nil
	}

	chartBarWidth := width / numBars

	return &histogramScale{
		from:               from,
		to:                 to,
		numDataBins:        numDataBins,
		dataBinsInChartBar: dataBinsInChartBar,
		chartBarWidth:      chartBarWidth,
	}
}

func (h *Histogram) fldDataToLines(dots [][]bool) []string {
	ret := make([]string, 0, len(dots)/2)

	for y := 0; y < len(dots); y += 2 {
		fldRow1 := dots[y+0]
		fldRow2 := dots[y+1]

		row := strings.Builder{}
		row.Grow(len(fldRow1))

		for x := 0; x < len(fldRow1); x += 2 {
			qblockID := 0
			if fldRow1[x+0] {
				qblockID |= (1 << 3)
			}
			if fldRow1[x+1] {
				qblockID |= (1 << 2)
			}
			if fldRow2[x+0] {
				qblockID |= (1 << 1)
			}
			if fldRow2[x+1] {
				qblockID |= (1 << 0)
			}

			row.WriteRune(qblocks[qblockID])
		}

		ret = append(ret, row.String())
	}

	return ret
}

func (h *Histogram) valToCoord(v int) int {
	return (v - h.from) / h.getDataBinsInChartBar() * h.getChartBarWidth() / h.binSize
}

func clearTviewFormatting(input string) string {
	var output strings.Builder
	inTag := false
	escaped := false
	bracketDepth := 0

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inTag {
			if c == ']' {
				if escaped {
					// Close escaped tag like [red[] -> output [red]
					output.WriteString("[" + input[bracketDepth:i-1] + "]")
					inTag = false
					escaped = false
				} else {
					// Close of a formatting directive
					inTag = false
				}
			} else if c == '[' && i+1 < len(input) && input[i+1] == ']' {
				// Detected a pattern like [red[] or ["tagname"[] etc.
				escaped = true
			}
			continue
		}

		if c == '[' {
			// Look ahead to check for [[] (just a literal open bracket)
			if i+1 < len(input) && input[i+1] == '[' {
				output.WriteByte('[')
				i++ // skip next '['
				continue
			}

			// Start of a potential formatting tag
			inTag = true
			bracketDepth = i + 1
			continue
		}

		// Normal character
		output.WriteByte(c)
	}

	return output.String()
}
