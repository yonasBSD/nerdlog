package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectQuery(t *testing.T) {
	type testCase struct {
		descr string

		// str is the string select query that we try to parse.
		str SelectQuery

		// If strRemarshaled is not empty, it means that after we remarshal the query,
		// the result is expected to be different from str.
		strRemarshaled SelectQuery

		wantParsed *SelectQueryParsed
		wantErr    string
	}

	testCases := []testCase{
		testCase{descr: "close to default one", // {{{
			str: "time STICKY, message, lstream, level_name AS lvl, redacted_id_int AS ds",
			wantParsed: &SelectQueryParsed{
				Fields: []SelectQueryField{
					SelectQueryField{
						Name:        "time",
						DisplayName: "time",
						Sticky:      true,
					},
					SelectQueryField{
						Name:        "message",
						DisplayName: "message",
					},
					SelectQueryField{
						Name:        "lstream",
						DisplayName: "lstream",
					},
					SelectQueryField{
						Name:        "level_name",
						DisplayName: "lvl",
					},
					SelectQueryField{
						Name:        "redacted_id_int",
						DisplayName: "ds",
					},
				},
			},
		}, // }}}
		testCase{descr: "two sticky fields", // {{{
			str: "time STICKY, message, lstream, level_name AS lvl STICKY, redacted_id_int AS ds",
			wantParsed: &SelectQueryParsed{
				Fields: []SelectQueryField{
					SelectQueryField{
						Name:        "time",
						DisplayName: "time",
						Sticky:      true,
					},
					SelectQueryField{
						Name:        "message",
						DisplayName: "message",
					},
					SelectQueryField{
						Name:        "lstream",
						DisplayName: "lstream",
					},
					// NOTE that in the actual app it will be moved to the front (right after time),
					// but it's not done on the marshaling level, to avoid the remarshaled string
					// look differently.
					SelectQueryField{
						Name:        "level_name",
						DisplayName: "lvl",
						Sticky:      true,
					},
					SelectQueryField{
						Name:        "redacted_id_int",
						DisplayName: "ds",
					},
				},
			},
		}, // }}}
		testCase{descr: "close to default one, with a wildcard", // {{{
			str: "time STICKY, message, lstream, level_name AS lvl, redacted_id_int AS ds, *",
			wantParsed: &SelectQueryParsed{
				Fields: []SelectQueryField{
					SelectQueryField{
						Name:        "time",
						DisplayName: "time",
						Sticky:      true,
					},
					SelectQueryField{
						Name:        "message",
						DisplayName: "message",
					},
					SelectQueryField{
						Name:        "lstream",
						DisplayName: "lstream",
					},
					SelectQueryField{
						Name:        "level_name",
						DisplayName: "lvl",
					},
					SelectQueryField{
						Name:        "redacted_id_int",
						DisplayName: "ds",
					},
				},

				IncludeAll: true,
			},
		}, // }}}
		testCase{descr: "only wildcard", // {{{
			str: "*",
			wantParsed: &SelectQueryParsed{
				IncludeAll: true,
			},
		}, // }}}
		testCase{descr: "no uppercase", // {{{
			str:            "time sticky, message, lstream, level_name as lvl, redacted_id_int as ds",
			strRemarshaled: "time STICKY, message, lstream, level_name AS lvl, redacted_id_int AS ds",
			wantParsed: &SelectQueryParsed{
				Fields: []SelectQueryField{
					SelectQueryField{
						Name:        "time",
						DisplayName: "time",
						Sticky:      true,
					},
					SelectQueryField{
						Name:        "message",
						DisplayName: "message",
					},
					SelectQueryField{
						Name:        "lstream",
						DisplayName: "lstream",
					},
					SelectQueryField{
						Name:        "level_name",
						DisplayName: "lvl",
					},
					SelectQueryField{
						Name:        "redacted_id_int",
						DisplayName: "ds",
					},
				},
			},
		}, // }}}

		testCase{descr: "wildcard as a non-last item: error", // {{{
			str:     "time STICKY, message, lstream, level_name AS lvl, *, redacted_id_int AS ds",
			wantErr: "wildcard can only be the last item",
		}, // }}}
		testCase{descr: "more than one AS: error", // {{{
			str:     "time STICKY, message, lstream, level_name AS foo AS bar, redacted_id_int as ds",
			wantErr: "syntax error for field level_name: more than a single 'AS'",
		}, // }}}
		testCase{descr: "empty: error", // {{{
			str:     "",
			wantErr: "no fields selected",
		}, // }}}
	}

	for i, tc := range testCases {
		assertArgs := []interface{}{"test case #%d (%s)", i, tc.descr}

		gotParsed, gotErr := ParseSelectQuery(tc.str)

		assert.Equal(t, tc.wantParsed, gotParsed, assertArgs...)

		if tc.wantErr == "" {
			assert.Nil(t, gotErr, assertArgs...)
		} else {
			assert.Equal(t, tc.wantErr, gotErr.Error(), assertArgs...)
		}

		if gotParsed != nil {
			gotRemarshaled := gotParsed.Marshal()
			wantRemarshaled := tc.strRemarshaled
			if wantRemarshaled == "" {
				wantRemarshaled = tc.str
			}

			assert.Equal(t, wantRemarshaled, gotRemarshaled, assertArgs...)
		}
	}
}
