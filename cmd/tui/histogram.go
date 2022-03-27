package main

import (
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

	lines := h.fldDataToLines(fldData.dots)

	for lineY, line := range lines {
		tview.Print(screen, line, x, y+lineY, width, tview.AlignLeft, tcell.ColorLightGray)
	}

	marks := h.getXMarks(h.from, h.to, width-fldMarginLeft)

	sb := strings.Builder{}
	numRunes := 0

	for _, mark := range marks {
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

type fieldData struct {
	dots               [][]bool
	dataBinsInChartBin int
	chartBinsInDataBin int

	//effectiveWidth int
}

// genFieldData returns a 2-dimensional field as nested slices: [y][x].
// If the field is too small, returns nil.
func (h *Histogram) genFieldData(width, height int) *fieldData {
	rangeLen := (h.to - h.from + 1) / h.binSize

	if rangeLen == 0 {
		return nil
	}

	dataBinsInChartBin := (rangeLen + width - 1) / width
	chartBinsInDataBin := width / rangeLen
	if chartBinsInDataBin == 0 {
		chartBinsInDataBin = 1
	}

	//effectiveWidth := rangeLen

	// Find the max bin value
	max := 0
	for _, v := range h.data {
		if max < v {
			max = v
		}
	}

	dotSize := (max + height - 1) / height
	// TODO: round it

	// Allocate all the slices so we have the field ready
	dots := make([][]bool, height)
	for y := 0; y < height; y++ {
		dots[y] = make([]bool, width)
	}

	// Iterate over data and set dots to true
	for xData, xChart := 0, 0; xData < rangeLen; xData, xChart = xData+dataBinsInChartBin, xChart+chartBinsInDataBin {
		var val int
		for i := 0; i < dataBinsInChartBin; i++ {
			val += h.data[h.from+(xData+i)*h.binSize]
		}

		for y := 0; y < height; y++ {
			if val <= y*dotSize {
				break
			}

			for i := 0; i < chartBinsInDataBin; i++ {
				dots[height-y-1][xChart+i] = true
			}
		}
	}

	return &fieldData{
		dots:               dots,
		dataBinsInChartBin: dataBinsInChartBin,
		chartBinsInDataBin: chartBinsInDataBin,
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
	return (v - h.from) / fldData.dataBinsInChartBin * fldData.chartBinsInDataBin
}
