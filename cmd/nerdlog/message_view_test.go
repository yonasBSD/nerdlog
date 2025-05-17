package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOptimalMessageViewSize(t *testing.T) {
	tests := []struct {
		name           string
		screenWidth    int
		extraWidth     int
		extraHeight    int
		text           string
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "Single short line fits in screen",
			screenWidth:    80,
			extraWidth:     2,
			extraHeight:    1,
			text:           "hello world",
			expectedWidth:  len("hello world") + 2,
			expectedHeight: 1 + 1,
		},
		{
			name:           "Long line wraps",
			screenWidth:    10,
			extraWidth:     2,
			extraHeight:    1,
			text:           "123456789012345",
			expectedWidth:  10,
			expectedHeight: 1 + 2, // two lines on 8-width screen
		},
		{
			name:           "Multiple lines mixed wrapping",
			screenWidth:    10,
			extraWidth:     1,
			extraHeight:    2,
			text:           "short\n1234567890123\nok",
			expectedWidth:  10,
			expectedHeight: 2 + 1 + 2 + 1, // short:1, long:2, ok:1
		},
		{
			name:           "Empty string",
			screenWidth:    10,
			extraWidth:     2,
			extraHeight:    1,
			text:           "",
			expectedWidth:  2,
			expectedHeight: 1 + 1, // one empty line still counts as one
		},
		{
			name:           "Zero screen width",
			screenWidth:    0,
			extraWidth:     3,
			extraHeight:    2,
			text:           "hello",
			expectedWidth:  0,
			expectedHeight: 2 + 0,
		},
		{
			name:        "Long text with empty lines in between",
			screenWidth: 40,
			extraWidth:  4,
			extraHeight: 5,
			text: `Feidsld lsdkfjw ad asdfajksfka; asdflkj kdjfh asjdfhsjdf q;wlkx asdfkd qpdkj

asdfj qdfkqdjfa;slkdfj asdqd
asdlfkj skaj;lksdfja
a;slkdfj;alskd ;alkdjf;aslkdjqd


a;sldkfa al;ksdfj a;lskfj;aslkdfjaslkdjf aksjdhfkasjdhflkashdfkqjsdlfkjas asldfj alskdfjas da;lsdf

asdflasdf`,
			expectedWidth:  40,
			expectedHeight: 19,
		},
		{
			name:           "Text with a single trailing newline",
			screenWidth:    80,
			extraWidth:     4,
			extraHeight:    5,
			text:           "Some message some message\nSecond line\n",
			expectedWidth:  29,
			expectedHeight: 7,
		},
		{
			name:           "Text with lots of leading and trailing newlines and whitespace",
			screenWidth:    80,
			extraWidth:     4,
			extraHeight:    5,
			text:           "\n\n\n\n   \n\n \nSome message some message\nSecond line\n\n\n\n\n   \n          \n\n",
			expectedWidth:  29,
			expectedHeight: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := GetOptimalMessageViewSize(tt.screenWidth, tt.extraWidth, tt.extraHeight, tt.text)
			assert.Equal(t, tt.expectedWidth, width)
			assert.Equal(t, tt.expectedHeight, height)
		})
	}
}
