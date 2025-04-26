package main

// RuneBuffer allows writing strings at arbitrary positions, expanding as needed
type RuneBuffer struct {
	data []rune
}

// WriteAt writes a string at the given rune index, expanding the buffer if needed
func (b *RuneBuffer) WriteAt(index int, s string) {
	runes := []rune(s)
	end := index + len(runes)

	// Expand the buffer if necessary
	if end > len(b.data) {
		newData := make([]rune, end)
		copy(newData, b.data)
		b.data = newData
	}

	copy(b.data[index:end], runes)
}

func (b *RuneBuffer) Rune(index int) (rune, bool) {
	if index >= 0 && index < len(b.data) {
		return b.data[index], true
	}
	return ' ', false
}

// String returns the current buffer as a string
func (b *RuneBuffer) String() string {
	return string(b.data)
}
