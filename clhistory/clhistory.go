package clhistory

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/juju/errors"
)

type CLHistory struct {
	params CLHistoryParams

	items []Item

	// curHistIdx is used when navigating the history using Prev / Next.
	// When navigating isn't in progress (after a new item was added using Add),
	// it's reset to -1.
	curHistIdx        int
	lastEphemeralItem Item
}

type CLHistoryParams struct {
	// Filename is where to load the history from and write it to.  If it's
	// empty, the history is only kept in RAM and not persisted anywhere.
	Filename string
}

type Item struct {
	Time time.Time

	Str string
}

func New(params CLHistoryParams) *CLHistory {
	h := &CLHistory{
		params: params,

		curHistIdx: -1,
	}

	h.Load()

	return h
}

// Load loads all history from the file. If Filename in params is empty,
// Load is a no-op.
func (h *CLHistory) Load() {
	if h.params.Filename == "" {
		return
	}

	// TODO
}

// Add adds the given string as a new history item to the in-RAM history and,
// if Filename in params was not empty, then also to this file. It also resets
// the history navigation, if any.
func (h *CLHistory) Add(s string) error {
	h.resetHistoryNavigation()

	item := Item{
		Time: time.Now(),
		Str:  s,
	}

	h.items = append(h.items, item)

	if h.params.Filename != "" {
		f, err := os.OpenFile(h.params.Filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return errors.Trace(err)
		}

		defer f.Close()

		fmt.Fprint(f, string(marshalItem(item)))
	}

	return nil
}

// Reset resets the history navigation. Typically client code should call it
// when a user edits or aborts/accepts the command line.
func (h *CLHistory) Reset() {
	h.resetHistoryNavigation()
}

// Prev returns what it considers the previous item.
func (h *CLHistory) Prev(s string) Item {
	if h.curHistIdx == -1 {
		h.startHistoryNavigation(s)
	}

	h.curHistIdx--
	if h.curHistIdx < 0 {
		h.curHistIdx = 0
	}

	return h.getItem(h.curHistIdx)
}

// Next returns what it considers the next item.
func (h *CLHistory) Next(s string) Item {
	if h.curHistIdx == -1 {
		h.startHistoryNavigation(s)
	}

	h.curHistIdx++
	if h.curHistIdx > len(h.items) {
		// We do allow it to exceed the data by 1 item, which means just returning
		// lastEphemeralItem; thus we use len(h.items) and not len(h.items)-1.
		h.curHistIdx = len(h.items)
	}

	return h.getItem(h.curHistIdx)
}

func (h *CLHistory) startHistoryNavigation(s string) {
	h.curHistIdx = len(h.items)
	h.lastEphemeralItem = Item{Str: s}
}

func (h *CLHistory) resetHistoryNavigation() {
	h.curHistIdx = -1
	h.lastEphemeralItem = Item{}
}

func (h *CLHistory) getItem(idx int) Item {
	if idx < len(h.items) {
		return h.items[idx]
	}

	if idx == len(h.items) {
		return h.lastEphemeralItem
	}

	panic(fmt.Sprintf("idx=%d, len(items)=%d", idx, len(h.items)))
}

// :1650712458000000000:12:0:foo bar baz
func marshalItem(item Item) []byte {
	b := bytes.Buffer{}
	b.WriteRune(':')
	b.WriteString(strconv.FormatInt(item.Time.UnixNano(), 10))
	b.WriteRune(':')
	b.WriteString(strconv.Itoa(len(item.Str) + 1))
	b.WriteString(":0:") // For now, no extra info
	b.WriteString(item.Str)
	b.WriteRune('\n')

	return b.Bytes()
}
