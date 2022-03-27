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

	// xFormat formats values for X axis
	xFormat func(v int) string
}

func NewHistogram() *Histogram {
	return &Histogram{
		Box: tview.NewBox(),
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

func (h *Histogram) SetXFormat(xFormat func(v int) string) *Histogram {
	h.xFormat = xFormat

	return h
}

func (h *Histogram) Draw(screen tcell.Screen) {
	h.Box.DrawForSubclass(screen, h)
	x, y, width, height := h.GetInnerRect()

	// We multiply width and height by 2 because we use quadrant graphics,
	// so one character is a 2x2 field.
	fldWidth := width * 2
	fldHeight := (height - 1) * 2 // One line for axis

	fldData := h.genFieldData(fldWidth, fldHeight)
	if fldData == nil {
		// Field is too small
		return
	}

	lines := h.fldDataToLines(fldData)

	for lineY, line := range lines {
		tview.Print(screen, line, x, y+lineY, width, tview.AlignLeft, tcell.ColorWhite)
	}

	//for index, option := range h.options {
	//if index >= height {
	//break
	//}
	//radioButton := "\u25ef" // Unchecked.
	//if index == h.currentOption {
	//radioButton = "\u25c9" // Checked.
	//}
	//line := fmt.Sprintf(`%s[white]  %s`, radioButton, option)
	//tview.Print(screen, line, x, y+index, width, tview.AlignLeft, tcell.ColorYellow)
	//}
}

// genFieldData returns a 2-dimensional field as nested slices: [y][x].
// If the field is too small, returns nil.
func (h *Histogram) genFieldData(width, height int) [][]bool {
	rangeLen := (h.to - h.from + 1) / h.binSize

	dataBinsInChartBin := (rangeLen + width - 1) / width

	// Find the max bin value
	max := 0
	for _, v := range h.data {
		if max < v {
			max = v
		}
	}

	dotSize := (max + height - 1) / height
	//fmt.Println("HEY dotSize", dotSize)
	// TODO: round it

	// Allocate all the slices so we have the field ready
	ret := make([][]bool, height)
	for y := 0; y < height; y++ {
		ret[y] = make([]bool, width)
	}

	for xData, xChart := 0, 0; xData < rangeLen; xData, xChart = xData+dataBinsInChartBin, xChart+1 {
		var val int
		for i := 0; i < dataBinsInChartBin; i++ {
			val += h.data[h.from+(xData+i)*h.binSize]
		}

		for y := 0; y < height; y++ {
			if val > y*dotSize {
				ret[height-y-1][xChart] = true
			}
		}
	}

	return ret

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

func (h *Histogram) fldDataToLines(fldData [][]bool) []string {
	ret := make([]string, 0, len(fldData)/2)

	for y := 0; y < len(fldData); y += 2 {
		fldRow1 := fldData[y+0]
		fldRow2 := fldData[y+1]

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
