package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClearTviewFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple color", "[yellow]Yellow text", "Yellow text"},
		{"Background color", "[yellow:red]Yellow text on red background", "Yellow text on red background"},
		{"Only background", "[:red]Red background", "Red background"},
		{"Underline", "[yellow::u]Underlined text", "Underlined text"},
		{"Bold and blinking", "[::bl]Bold blinking", "Bold blinking"},
		{"Reset styles", "[::-]Text after reset", "Text after reset"},
		{"Reset foreground", "[-]No color", "No color"},
		{"Italic on", "[::i]Italic text", "Italic text"},
		{"Italic off", "[::I]Normal text", "Normal text"},
		{"Link formatting", "Click [:::https://example.com]here[:::-] for more", "Click here for more"},
		{"Multiple mailto links", "Email [:::mailto:a@x]a/[:::mail:b@x]b/[:::mail:c@x]c[:::-]", "Email a/b/c"},
		{"Reset everything", "[-:-:-:-]Clean", "Clean"},
		{"No-op tag", "[:]Still here", "Still here"},
		{"Invalid tag shows brackets", "[]Bracket tag", "Bracket tag"},
		{"Escaped tag", "[red[]Text", "[red]Text"},
		{"Escaped quoted tag", "[\"123\"[]hello", "[\"123\"]hello"},
		{"Escaped color tag", "[#6aff00[[]Greenish", "[#6aff00[]Greenish"},
		{"Escaped nonsense", "[a#\"[[[]stuff", "[a#\"[[]stuff"},
		{"Mixed content", "Start [yellow]middle[::u]under[:::-]end", "Start middleunderend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clearTviewFormatting(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetOptimalScale_Custom(t *testing.T) {
	const binSize = 60 // 1 minute

	tests := []struct {
		name     string
		from     int
		to       int
		width    int
		expected *histogramScale
	}{
		{
			name:  "BasicCase",
			from:  0,
			to:    3600,
			width: 100,
			expected: &histogramScale{
				from:               0,
				to:                 3600,
				numDataBins:        60,
				dataBinsInChartBar: 1,
				chartBarWidth:      1,
			},
		},
		{
			name:  "SnappingUp",
			from:  0,
			to:    60*18*100 - 2,
			width: 100,
			expected: &histogramScale{
				from:               0,
				to:                 60*18*100 - 0,
				numDataBins:        1800,
				dataBinsInChartBar: 20,
				chartBarWidth:      1,
			},
		},
		{
			name:  "ExactSnapTo2",
			from:  0,
			to:    3600,
			width: 30,
			expected: &histogramScale{
				from:               0,
				to:                 3600,
				numDataBins:        60,
				dataBinsInChartBar: 2,
				chartBarWidth:      1,
			},
		},
		{
			name:  "NonAlignedInputWithSnapTo5",
			from:  1000065,
			to:    1001130,
			width: 100,
			expected: &histogramScale{
				from:               1000020,
				to:                 1001160,
				numDataBins:        19,
				dataBinsInChartBar: 1,
				chartBarWidth:      5,
			},
		},
		{
			name:  "WideRangeMinimalWidth",
			from:  0,
			to:    60 * 10000,
			width: 1,
			expected: &histogramScale{
				from:               0,
				to:                 60 * 10080,
				numDataBins:        10080,
				dataBinsInChartBar: 10080,
				chartBarWidth:      1,
			},
		},
		{
			name:     "TooSmallToHaveABin",
			from:     0,
			to:       59,
			width:    10,
			expected: nil,
		},
		{
			name:  "chart bar width increases to 2",
			from:  0,
			to:    660 * 60,
			width: 320,
			expected: &histogramScale{
				from:               0,
				to:                 660 * 60,
				numDataBins:        660,
				dataBinsInChartBar: 5,
				chartBarWidth:      2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getOptimalScale(tt.from, tt.to, binSize, tt.width, snapDataBinsInChartDot)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

// TestGetOptimalScale_Random uses random inputs, and makes sure that the
// output always satisifies the following conditions:
//
//   - The resulting chart width never exceeds the given width
//   - If we increase chartBarWidth by just 1, it would already exceed the chart
//     width
//   - The dataBinsInChartBar is snapped (snapDataBinsInChartDot returns the same
//     value)
//   - If we try to increase resolution by reducing dataBinsInChartBar to the next
//     smallest snap, and set chartBarWidth to 1, we would exceed the chart width
func TestGetOptimalScale_Random(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	const binSize = 100
	const iterations = 200000

	for i := 0; i < iterations; i++ {
		from := rand.Intn(1000)                     // 0–999
		duration := (rand.Intn(1440) + 1) * binSize // 1 min to 24 hrs
		to := from + duration
		width := rand.Intn(2000) + 1

		scale := getOptimalScale(from, to, binSize, width, snapDataBinsInChartDot)
		if scale == nil {
			continue
		}

		numBars := scale.numDataBins / scale.dataBinsInChartBar
		totalChartWidth := numBars * scale.chartBarWidth

		detailsStr := fmt.Sprintf(
			"inputs: from=%d, to=%d, width=%d; outputs=%+v (numBars=%d, totalWidth=%d)",
			from, to, width,
			scale,
			numBars, totalChartWidth,
		)

		// 1. Total chart width must not exceed the available width
		assert.LessOrEqual(t, totalChartWidth, width,
			fmt.Sprintf("total chart width should not exceed the provided width. %s", detailsStr))

		// 2. If we increase chartBarWidth by 1, total width would exceed provided width
		extraWidth := numBars * (scale.chartBarWidth + 1)
		assert.Greater(t, extraWidth, width,
			fmt.Sprintf("increasing chartBarWidth by 1 should exceed the available width. %s", detailsStr),
		)

		// 3. The dataBinsInChartBar is correctly snapped
		snapped := snapDataBinsInChartDot(scale.dataBinsInChartBar)
		assert.Equal(t, scale.dataBinsInChartBar, snapped,
			fmt.Sprintf("dataBinsInChartBar should match the snapped value. %s", detailsStr),
		)

		// 4. If we reduce dataBinsInChartBar to the next smallest snap and set chartBarWidth = 1,
		//    total chart width should exceed available width
		var prevSnap int
		for _, snap := range snaps {
			snapMinutes := int(snap / time.Minute)
			if snapMinutes >= scale.dataBinsInChartBar {
				break
			}
			prevSnap = snapMinutes
		}

		if prevSnap > 0 {
			numBarsAtHigherRes := scale.numDataBins / prevSnap
			if scale.numDataBins%prevSnap != 0 {
				numBarsAtHigherRes++
			}
			higherResTotalWidth := numBarsAtHigherRes * 1
			assert.Greater(t, higherResTotalWidth, width,
				"trying smaller dataBinsInChartBar at chartBarWidth=1 should exceed width")
		}
	}
}
