package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func formatRFC3339Slice(vals []time.Time) []string {
	var result []string
	for _, v := range vals {
		s := v.Format(time.RFC3339)
		result = append(result, s)
	}
	return result
}

func TestGetXMarksForTimeRange(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		maxMarks int
		expected []string
	}{
		{
			name:     "1-hour range, 10 marks",
			from:     "2023-01-01T12:00:00Z",
			to:       "2023-01-01T13:00:00Z",
			maxMarks: 10,
			expected: []string{
				"2023-01-01T12:00:00Z",
				"2023-01-01T12:10:00Z",
				"2023-01-01T12:20:00Z",
				"2023-01-01T12:30:00Z",
				"2023-01-01T12:40:00Z",
				"2023-01-01T12:50:00Z",
				"2023-01-01T13:00:00Z",
			},
		},
		{
			name:     "24-hour range, 5 marks",
			from:     "2023-01-01T00:00:00Z",
			to:       "2023-01-02T00:00:00Z",
			maxMarks: 5,
			expected: []string{
				"2023-01-01T00:00:00Z",
				"2023-01-01T06:00:00Z",
				"2023-01-01T12:00:00Z",
				"2023-01-01T18:00:00Z",
				"2023-01-02T00:00:00Z",
			},
		},
		{
			name:     "10-day range, 7 marks",
			from:     "2023-01-01T00:00:00Z",
			to:       "2023-01-11T00:00:00Z",
			maxMarks: 7,
			expected: []string{
				"2023-01-01T00:00:00Z",
				"2023-01-03T00:00:00Z",
				"2023-01-05T00:00:00Z",
				"2023-01-07T00:00:00Z",
				"2023-01-09T00:00:00Z",
				"2023-01-11T00:00:00Z",
			},
		},
		{
			name:     "30-minute range, 4 marks",
			from:     "2023-01-01T10:00:00Z",
			to:       "2023-01-01T10:30:00Z",
			maxMarks: 4,
			expected: []string{
				"2023-01-01T10:00:00Z",
				"2023-01-01T10:10:00Z",
				"2023-01-01T10:20:00Z",
				"2023-01-01T10:30:00Z",
			},
		},
		{
			name:     "Invalid range",
			from:     "2023-01-02T00:00:00Z",
			to:       "2023-01-01T00:00:00Z",
			maxMarks: 5,
			expected: nil,
		},

		{
			name:     "Odd 1h17m range, 5 marks",
			from:     "2023-01-01T09:13:00Z",
			to:       "2023-01-01T10:30:00Z",
			maxMarks: 5,
			expected: []string{
				"2023-01-01T09:15:00Z",
				"2023-01-01T09:30:00Z",
				"2023-01-01T09:45:00Z",
				"2023-01-01T10:00:00Z",
				"2023-01-01T10:15:00Z",
			},
		},
		{
			name:     "Odd 2h37m range, 6 marks",
			from:     "2023-01-01T03:27:00Z",
			to:       "2023-01-01T06:04:00Z",
			maxMarks: 6,
			expected: []string{
				"2023-01-01T03:30:00Z",
				"2023-01-01T04:00:00Z",
				"2023-01-01T04:30:00Z",
				"2023-01-01T05:00:00Z",
				"2023-01-01T05:30:00Z",
				"2023-01-01T06:00:00Z",
			},
		},
		{
			name:     "3-day non-midnight range",
			from:     "2023-01-01T08:20:00Z",
			to:       "2023-01-04T17:40:00Z",
			maxMarks: 7,
			expected: []string{
				"2023-01-01T12:00:00Z",
				"2023-01-02T00:00:00Z",
				"2023-01-02T12:00:00Z",
				"2023-01-03T00:00:00Z",
				"2023-01-03T12:00:00Z",
				"2023-01-04T00:00:00Z",
				"2023-01-04T12:00:00Z",
			},
		},
		{
			name:     "2-day non-midnight range",
			from:     "2023-01-01T08:20:00Z",
			to:       "2023-01-03T17:40:00Z",
			maxMarks: 7,
			expected: []string{
				"2023-01-01T12:00:00Z",
				"2023-01-02T00:00:00Z",
				"2023-01-02T12:00:00Z",
				"2023-01-03T00:00:00Z",
				"2023-01-03T12:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, _ := time.Parse(time.RFC3339, tt.from)
			to, _ := time.Parse(time.RFC3339, tt.to)

			actual := getXMarksForTimeRange(time.UTC, from, to, tt.maxMarks)
			actualStrs := formatRFC3339Slice(actual)

			assert.Equal(t, tt.expected, actualStrs)
		})
	}
}
