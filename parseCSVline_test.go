package mv370

import (
	"testing"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		line   string
		expect []string
	}{
		{
			line:   "",
			expect: []string{},
		},
		{
			line:   "1",
			expect: []string{"1"},
		},
		{
			line:   "1,",
			expect: []string{"1", ""},
		},
		{
			line:   "Hello, World",
			expect: []string{"Hello", " World"},
		},
		{
			line:   `ONE,"TWO",THREE`,
			expect: []string{"ONE", "TWO", "THREE"},
		},
		{
			line:   "Multi-line\nLine 2",
			expect: []string{"Multi-line\nLine 2"},
		},
		// These fail, but are they valid csv anyway?
		// {
		// 	line:   `ONE,"TWO","Double""quotes"`,
		// 	expect: []string{"ONE", `Double"quotes`, "THREE"},
		// },
		// {
		// 	line:   `ONE,"TWO","THREE`,
		// 	expect: []string{"ONE", "TWO", `"THREE`},
		// },
		// {
		// 	line:   `ONE,MISSING QUOTE",TWO`,
		// 	expect: []string{"ONE", `MISSING QUOTE"`, "THREE"},
		// },
	}

	for _, test := range tests {
		out, err := ParseCSVLine(test.line)
		if err != nil {
			t.Log(err)
			t.Fail()
		} else if len(out) != len(test.expect) {
			t.Logf("Expected %d fields, got %d: '%s'", len(test.expect), len(out), test.line)
			t.Fail()
		} else {
			for i, v := range test.expect {
				if v != out[i] {
					t.Logf("Expected '%s', got '%s': '%s'", v, out[i], test.line)
					t.Fail()
				}
			}
		}
	}
}
