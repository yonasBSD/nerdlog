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

	dataBinsInChartDot int
	chartDotsInDataBin int
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
	h.cursor = h.to - h.binSize
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

	h.dataBinsInChartDot = fldData.dataBinsInChartDot
	h.chartDotsInDataBin = fldData.chartDotsInDataBin

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
		dotCoord := h.valToCoord(fldData, mark) / 60

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
}

func (h *Histogram) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return h.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		maxCursor := h.to - h.binSize

		moveLeft := func() {
			h.cursor -= h.binSize
			if h.cursor < h.from {
				h.cursor = h.from
			}
		}

		moveRight := func() {
			h.cursor += h.binSize
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
				case 'v', ' ':
					selectionApplyIfActive()
					selectionToggle()
				case 'o':
					if h.selectionStart > 0 {
						h.cursor, h.selectionStart = h.selectionStart, h.cursor
					}
				}
			}

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

	selEnd += h.dataBinsInChartDot * h.binSize

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

	/*
		isCursorAt := func(idx, n int) bool {
			for i := 0; i < n; i++ {
				if h.cursor == h.from+(idx+i)*h.binSize {
					return true
				}
			}

			return false
		}
	*/

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

	// Iterate over data and set dots to true
	for xData, xChart := 0, 0; xData < rangeLen; xData, xChart = xData+dataBinsInChartDot, xChart+chartDotsInDataBin {
		val := valAt(xData, dataBinsInChartDot)
		crs := foc && isSelectedAt(xData, dataBinsInChartDot)

		for y := 0; y < height; y++ {
			on := val > y*dotYScale

			// As an optimization: if the dot is off and the cursor is not here, it
			// means that all other dots in this column will be off, so we're done
			// with this column.
			if !on && !crs {
				break
			}

			// If cursor is here, we need to inverse the dot.
			if crs {
				on = !on
			}

			// If the dots are on, set them to true.
			if on {
				for i := 0; i < chartDotsInDataBin; i++ {
					dots[height-y-1][xChart+i] = true
				}
			}
		}
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

func (h *Histogram) valToCoord(fldData *fieldData, v int) int {
	return (v - h.from) / fldData.dataBinsInChartDot * fldData.chartDotsInDataBin
}
