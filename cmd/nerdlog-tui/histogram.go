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
	h.cursor = h.to - h.binSize*h.getDataBinsInChartDot()
	h.selectionStart = 0

	return h
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

func (h *Histogram) SetXMarker(getXMarks func(from, to int, numChars int) []int) *Histogram {
	h.getXMarks = getXMarks

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
		numRunes += len(markStr)
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
			selMark = fmt.Sprintf(" [%s]", h.xFormat(h.cursor))
		} else {
			selStart, selEnd := h.GetSelection()
			selMark = fmt.Sprintf(" [%s - %s]", h.xFormat(selStart), h.xFormat(selEnd))
		}

		leftOffset := fldMarginLeft + fldData.selScaleOffset/2

		tview.Print(screen, line+selMark, x+leftOffset, y+height-1, width-leftOffset, tview.AlignLeft, tcell.ColorGreen)

		// Also print the bar value (or sum of all selected values, if selection is active)
		var valToPrint string
		if !h.IsSelectionActive() {
			valToPrint = fmt.Sprintf("(%d)", fldData.cursorVal)
		} else {
			valToPrint = fmt.Sprintf("(total %d)", fldData.selectedValsSum)
		}
		tview.Print(screen, valToPrint, x+leftOffset+lineLen+1, y, width-leftOffset-lineLen-1, tview.AlignLeft, tcell.ColorGreen)
	}
}

func (h *Histogram) getDataBinsInChartDot() int {
	if h.fldData == nil {
		return 1
	}

	return h.fldData.dataBinsInChartDot
}

func (h *Histogram) getChartDotsInDataBin() int {
	if h.fldData == nil {
		return 1
	}

	return h.fldData.chartDotsInDataBin
}

func (h *Histogram) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return h.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		maxCursor := h.to - h.binSize*h.getDataBinsInChartDot()

		moveLeft := func() {
			h.cursor -= h.binSize * h.getDataBinsInChartDot()
			if h.cursor < h.from {
				h.cursor = h.from
			}
		}

		moveRight := func() {
			h.cursor += h.binSize * h.getDataBinsInChartDot()
			if h.cursor > maxCursor {
				h.cursor = maxCursor
			}
		}

		moveLeftLong := func() {
			moveLeft()

			for h.cursor > h.from {
				for _, mark := range h.curMarks {
					if h.cursor == mark {
						return
					}
				}

				moveLeft()
			}
		}

		moveRightLong := func() {
			moveRight()

			for h.cursor < maxCursor {
				for _, mark := range h.curMarks {
					if h.cursor == mark {
						return
					}
				}

				moveRight()
			}
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

	selEnd += h.binSize * h.getDataBinsInChartDot()

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

	dataBinsInChartDot int
	chartDotsInDataBin int

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

	rangeLen := (h.to - h.from + 1) / h.binSize

	if rangeLen == 0 {
		return nil
	}

	dataBinsInChartDot := (rangeLen + width - 1) / width
	chartDotsInDataBin := width / rangeLen
	if chartDotsInDataBin == 0 {
		chartDotsInDataBin = 1
	}

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
		selEnd = h.cursor + dataBinsInChartDot*h.binSize
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

	//effectiveWidthDots := rangeLen

	// Find the max bin value per bar on the chart
	max := 0
	for xData := 0; xData < rangeLen; xData = xData + dataBinsInChartDot {
		val := valAt(xData, dataBinsInChartDot)

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

	selOffsetStart := -1
	selOffsetEnd := -1

	cursorVal := 0
	selectedValsSum := 0

	// Iterate over data and set dots to true
	for xData, xChart := 0, 0; xData < rangeLen; xData, xChart = xData+dataBinsInChartDot, xChart+chartDotsInDataBin {
		val := valAt(xData, dataBinsInChartDot)
		sel := isSelectedAt(xData, dataBinsInChartDot)
		crs := isCursorAt(xData, dataBinsInChartDot)

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
				for i := 0; i < chartDotsInDataBin; i++ {
					dots[height-y-1][xChart+i] = true
				}
			}
		}

		for i := 0; i < chartDotsInDataBin; i++ {
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
				selOffsetEnd = (xChart + i)
				if (selOffsetEnd & 0x01) != 0 {
					selOffsetEnd += 1
				}
			}

			if crs {
				selScaleDots[1][xChart+i] = true
			}
		}
	}

	// Cut the empty data from selScaleDots
	for i := range selScaleDots {
		if selOffsetEnd != -1 {
			selScaleDots[i] = selScaleDots[i][:selOffsetEnd]
		}
		selScaleDots[i] = selScaleDots[i][selOffsetStart:]
	}

	effectiveWidthDots := rangeLen / dataBinsInChartDot * chartDotsInDataBin
	effectiveWidthRunes := effectiveWidthDots / 2
	if (effectiveWidthDots & 0x01) > 0 {
		effectiveWidthRunes++
	}

	return &fieldData{
		dots:               dots,
		dataBinsInChartDot: dataBinsInChartDot,
		chartDotsInDataBin: chartDotsInDataBin,

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
	return (v - h.from) / h.getDataBinsInChartDot() * h.getChartDotsInDataBin() / h.binSize
}
