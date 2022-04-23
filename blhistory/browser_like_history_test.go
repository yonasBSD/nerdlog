package blhistory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	add  string
	prev bool
	next bool

	want string
}

func TestBLHistory(t *testing.T) {
	testCases := []testCase{
		testCase{prev: true, want: ""},
		testCase{prev: true, want: ""},
		testCase{next: true, want: ""},
		testCase{next: true, want: ""},
		testCase{add: "item 1"},
		testCase{prev: true, want: ""},
		testCase{next: true, want: ""},
		testCase{add: "item 2"},
		testCase{prev: true, want: "item 1"},
		testCase{prev: true, want: ""},
		testCase{next: true, want: "item 2"},
		testCase{next: true, want: ""},
		testCase{add: "item 3"},
		testCase{prev: true, want: "item 2"},
		testCase{prev: true, want: "item 1"},
		testCase{prev: true, want: ""},
		testCase{prev: true, want: ""},
		testCase{next: true, want: "item 2"},
		testCase{next: true, want: "item 3"},
		testCase{next: true, want: ""},
		testCase{next: true, want: ""},
		testCase{next: true, want: ""},
		testCase{prev: true, want: "item 2"},
		testCase{add: "item 10"},
		testCase{next: true, want: ""},
		testCase{prev: true, want: "item 2"},
		testCase{prev: true, want: "item 1"},
		testCase{prev: true, want: ""},
		testCase{next: true, want: "item 2"},
		testCase{next: true, want: "item 10"},
		testCase{next: true, want: ""},
	}

	h := New()

	for i, tc := range testCases {
		if tc.add != "" {
			h.Add(tc.add)
		} else if tc.prev {
			var gotStr string
			got := h.Prev()
			if got != nil {
				gotStr = got.Str
			}

			assert.Equal(t, tc.want, gotStr, "testCase #%d (%+v)", i, tc)
		} else if tc.next {
			var gotStr string
			got := h.Next()
			if got != nil {
				gotStr = got.Str
			}

			assert.Equal(t, tc.want, gotStr, "testCase #%d (%+v)", i, tc)
		}
	}
}
