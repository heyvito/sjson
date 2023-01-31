package sjson

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func parseAll(data string) ([]byte, error) {
	p := Parser{}
	for _, b := range []byte(data) {
		data, err := p.Feed(b)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("failed: consumed all output, but got no result back")
}

func doParse(data string) (*Parser, []byte, error) {
	p := &Parser{}
	for _, b := range []byte(data) {
		data, err := p.Feed(b)
		if err != nil {
			return p, nil, err
		}
		if data != nil {
			return p, data, nil
		}
	}
	return p, nil, nil
}

func TestFalse(t *testing.T) {
	out, err := parseAll("false")
	require.NoError(t, err)
	assert.Equal(t, "false", string(out))
}

func TestTrue(t *testing.T) {
	out, err := parseAll("true")
	require.NoError(t, err)
	assert.Equal(t, "true", string(out))
}

func TestNull(t *testing.T) {
	out, err := parseAll("null")
	require.NoError(t, err)
	assert.Equal(t, "null", string(out))
}

func TestNumber(t *testing.T) {
	p, v, err := doParse("27")
	require.NoError(t, err)
	require.Empty(t, v)
	assert.Equal(t, []byte("27"), p.data)
}

func TestString(t *testing.T) {
	out, err := parseAll(`"test"`)
	require.NoError(t, err)
	assert.Equal(t, `"test"`, string(out))
}

func TestArray(t *testing.T) {
	tests := []string{"[]", "[[]]", "[1]", "[true]", "[false]", "[null]", "[\"hello\"]"}
	for _, v := range tests {
		t.Run("parses "+v, func(t *testing.T) {
			out, err := parseAll(v)
			require.NoError(t, err)
			assert.Equal(t, v, string(out))
		})
	}
}

func TestObject(t *testing.T) {
	tests := []string{"{}", `{"test":true}`, `{"test":[{"true":true,"array":[{"string":false}]}]}`}
	for _, v := range tests {
		t.Run("parses "+v, func(t *testing.T) {
			out, err := parseAll(v)
			require.NoError(t, err)
			assert.Equal(t, v, string(out))
		})
	}
}

func TestBorkedObject(t *testing.T) {
	_, _, err := doParse(`{"a":"a" 123}`)
	assert.Error(t, err)
}

func TestBorkedNumber(t *testing.T) {
	_, _, err := doParse(`123]`)
	assert.Error(t, err)
}

func TestHeterogeneousArray(t *testing.T) {
	_, _, err := doParse(`[null, 1, "1", {}]`)
	assert.NoError(t, err)
}

func TestArray1NewLine(t *testing.T) {
	_, _, err := doParse("[1\n]")
	assert.NoError(t, err)
}

func TestArray1eE2(t *testing.T) {
	_, _, err := doParse("[1eE2]")
	assert.Error(t, err)

}

func TestSuite(t *testing.T) {
	fixtures, err := os.ReadDir("fixtures")
	require.NoError(t, err)
	for _, v := range fixtures {
		if v.IsDir() {
			continue
		}
		n := v.Name()
		if !strings.HasSuffix(n, ".json") {
			continue
		}
		data, err := os.ReadFile("fixtures/" + n)
		require.NoError(t, err)
		expectedResult := ""
		switch {
		case strings.HasPrefix(n, "y_"):
			expectedResult = "parses"
		case strings.HasPrefix(n, "n_"):
			expectedResult = "fails"
		case strings.HasPrefix(n, "i_"):
			expectedResult = "is indifferent to"
		}

		t.Run(expectedResult+" "+n, func(t *testing.T) {
			_, err := parseAll(string(data))
			switch expectedResult {
			case "parses":
				require.NoError(t, err)
			case "fails":
				require.Error(t, err)
			}
		})
	}
}

func TestConsecutive(t *testing.T) {
	a := []byte(`[null,1,"1",{},false]`)
	b := []byte(`{"test":true}`)

	p := Parser{}
	var resultA []byte
	var resultB []byte
	var err error
	for _, v := range a {
		resultA, err = p.Feed(v)
		assert.NoError(t, err)
	}
	assert.Equal(t, a, resultA)

	for _, v := range b {
		resultB, err = p.Feed(v)
		assert.NoError(t, err)
	}
	assert.Equal(t, b, resultB)
	assert.Equal(t, a, resultA)
}
