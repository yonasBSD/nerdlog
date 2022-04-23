package shellescape

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type parseTC struct {
	shellCmd string
	want     []string
	wantErr  string
}

func TestParse(t *testing.T) {
	testCases := []parseTC{
		parseTC{shellCmd: ``, want: nil},
		parseTC{shellCmd: `foo bar bazzzz`, want: []string{`foo`, `bar`, `bazzzz`}},
		parseTC{shellCmd: `foo   bar      bazzzz`, want: []string{`foo`, `bar`, `bazzzz`}},
		parseTC{shellCmd: `  foo   bar      bazzzz    `, want: []string{`foo`, `bar`, `bazzzz`}},
		parseTC{shellCmd: `foo 'bar' bazzzz`, want: []string{`foo`, `bar`, `bazzzz`}},
		parseTC{shellCmd: `'foo' 'bar' 'bazzzz'`, want: []string{`foo`, `bar`, `bazzzz`}},
		parseTC{shellCmd: `' foo' 'bar  ' 'bazzzz'`, want: []string{` foo`, `bar  `, `bazzzz`}},
		parseTC{shellCmd: `foo 'bar bazz'zz`, want: []string{`foo`, `bar bazzzz`}},
		parseTC{shellCmd: `foo 'bar ba'"zz  z"z`, want: []string{`foo`, `bar bazz  zz`}},
		parseTC{shellCmd: `'foo bar bazzzz'`, want: []string{`foo bar bazzzz`}},
		parseTC{shellCmd: `"foo bar bazzzz"`, want: []string{`foo bar bazzzz`}},
		parseTC{shellCmd: `"foo bar" bazzzz`, want: []string{`foo bar`, `bazzzz`}},
		parseTC{shellCmd: `"foo bar"   bazzzz`, want: []string{`foo bar`, `bazzzz`}},
		parseTC{shellCmd: `"foo \"bar"   bazzzz`, want: []string{`foo "bar`, `bazzzz`}},
		parseTC{shellCmd: `"foo \" bar"   bazzzz`, want: []string{`foo " bar`, `bazzzz`}},
		parseTC{shellCmd: `"foo \\ bar"   bazzzz`, want: []string{`foo \ bar`, `bazzzz`}},
		parseTC{shellCmd: `'foo \" bar'   bazzzz`, want: []string{`foo \" bar`, `bazzzz`}},
		parseTC{shellCmd: `'foo '"'"'bar'"'"' baz'`, want: []string{`foo 'bar' baz`}},

		parseTC{shellCmd: `"foo \" bar   bazzzz`, wantErr: "unfinished quote"},
		parseTC{shellCmd: `'foo \" bar   bazzzz`, wantErr: "unfinished quote"},
	}

	for i, tc := range testCases {
		assertArgs := []interface{}{"testCase %d %q", i, tc.shellCmd}

		got, gotErr := Parse(tc.shellCmd)

		if tc.wantErr != "" {
			assert.Nil(t, got)
			assert.Equal(t, tc.wantErr, gotErr.Error(), assertArgs...)

			if gotErr != nil {
				assert.Equal(t, tc.wantErr, gotErr.Error(), assertArgs...)
			}
		} else {
			assert.Equal(t, tc.want, got, assertArgs...)
			assert.Nil(t, gotErr, assertArgs...)
		}
	}
}

type escapeTC struct {
	parts []string
	want  string
}

func TestEscape(t *testing.T) {
	testCases := []escapeTC{
		escapeTC{parts: nil, want: ""},
		escapeTC{parts: []string{`foo`, `bar`, `baz`}, want: `foo bar baz`},
		escapeTC{parts: []string{`foo13`, `bar`, `baz`}, want: `foo13 bar baz`},
		escapeTC{parts: []string{`foo bar`, `baz`}, want: `'foo bar' baz`},
		escapeTC{parts: []string{`foo "bar`, `baz`}, want: `'foo "bar' baz`},
		escapeTC{parts: []string{`'foo bar`, `baz`}, want: `''"'"'foo bar' baz`},
		escapeTC{parts: []string{`foo`, `bar, baz`}, want: `foo 'bar, baz'`},
	}

	for i, tc := range testCases {
		assertArgs := []interface{}{"testCase %d %q", i, tc.parts}

		got := Escape(tc.parts)
		assert.Equal(t, tc.want, got, assertArgs...)

		// Also make sure we can parse it back and get the same result
		gotParsed, gotParsedErr := Parse(got)
		assert.Equal(t, tc.parts, gotParsed, assertArgs...)
		assert.Nil(t, gotParsedErr, assertArgs...)
	}
}
