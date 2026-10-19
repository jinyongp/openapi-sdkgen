package probes

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type optional[T any] struct {
	value T
	set   bool
}

type nullable[T any] struct {
	value T
	valid bool
}

func some[T any](value T) optional[T]    { return optional[T]{value: value, set: true} }
func nonnull[T any](value T) nullable[T] { return nullable[T]{value: value, valid: true} }

func (value nullable[T]) MarshalJSON() ([]byte, error) {
	if !value.valid {
		return []byte("null"), nil
	}
	return json.Marshal(value.value)
}

func encodeField[T any](value optional[T]) ([]byte, error) {
	if !value.set {
		return []byte("{}"), nil
	}
	encoded, err := json.Marshal(value.value)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(`{"value":`), encoded...), '}'), nil
}

func TestPresenceEncoding(t *testing.T) {
	samples := []struct {
		name  string
		value optional[nullable[bool]]
		want  string
	}{
		{name: "absent", want: `{}`},
		{name: "null", value: some(nullable[bool]{}), want: `{"value":null}`},
		{name: "false", value: some(nonnull(false)), want: `{"value":false}`},
		{name: "true", value: some(nonnull(true)), want: `{"value":true}`},
	}
	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			data, err := encodeField(sample.value)
			if err != nil || string(data) != sample.want {
				t.Fatalf("encoded %s, error %v; want %s", data, err, sample.want)
			}
		})
	}
	for _, sample := range []struct{ data, want string }{{"", `{"value":""}`}, {"x", `{"value":"x"}`}} {
		data, err := encodeField(some(sample.data))
		if err != nil || string(data) != sample.want {
			t.Fatalf("string value: %s, %v", data, err)
		}
	}
	data, err := encodeField(some(0))
	if err != nil || string(data) != `{"value":0}` {
		t.Fatalf("zero value: %s, %v", data, err)
	}
	data, err = encodeField(some([]int{}))
	if err != nil || string(data) != `{"value":[]}` {
		t.Fatalf("empty array: %s, %v", data, err)
	}
}

func TestJSONBoundaryObservations(t *testing.T) {
	var target struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(`{"ID":"upper"}`), &target); err != nil || target.ID != "upper" {
		t.Fatalf("case-insensitive observation changed: %+v, %v", target, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"ID":"upper","extra":{"nested":true}}`), &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["id"]; ok {
		t.Fatal("raw key lookup unexpectedly ignored case")
	}
	if string(fields["extra"]) != `{"nested":true}` {
		t.Fatal("additional property was lost")
	}
	var invalid string
	if err := json.Unmarshal([]byte(`"\ud800"`), &invalid); err != nil || invalid != "\ufffd" {
		t.Fatalf("surrogate replacement observation changed: %q, %v", invalid, err)
	}
	output, err := exec.Command("node", "-e", `console.log(JSON.parse('"\\ud800"').charCodeAt(0).toString(16))`).Output()
	if err != nil || string(bytes.TrimSpace(output)) != "d800" {
		t.Fatalf("Node surrogate observation: %s, %v", output, err)
	}
	if utf8.RuneCountInString("😀") != 1 {
		t.Fatal("code-point count differs")
	}
	t.Log("standard struct decoding folds key case; raw fields preserve extra values; standard JSON replaces lone surrogate while Node preserves it")
}

func TestBinary64Compatibility(t *testing.T) {
	literals := []string{"0", "-0", "0.3", "0.30000000000000004", "9007199254740991", "9007199254740993", "1e21", "1e-7", "1e100"}
	input, _ := json.Marshal(literals)
	command := exec.Command("node", "-e", `let s="";
process.stdin.on("data", c => s += c);
process.stdin.on("end", () => {
  const values = JSON.parse(s).map(x => JSON.stringify(Number(x)));
  console.log(JSON.stringify(values));
});`)
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node numeric oracle: %v: %s", err, output)
	}
	var want []string
	if err := json.Unmarshal(output, &want); err != nil {
		t.Fatal(err)
	}
	for index, literal := range literals {
		number, err := strconv.ParseFloat(literal, 64)
		if err != nil {
			t.Fatal(err)
		}
		if number == 0 {
			number = 0
		}
		data, err := json.Marshal(number)
		if err != nil || string(data) != want[index] {
			t.Errorf("%s: Go=%s Node=%s error=%v", literal, data, want[index], err)
		}
	}
	value, _ := new(big.Rat).SetString(strconv.FormatFloat(0.3, 'g', -1, 64))
	divisor, _ := new(big.Rat).SetString(strconv.FormatFloat(0.1, 'g', -1, 64))
	quotient := new(big.Rat).Quo(value, divisor)
	if !quotient.IsInt() || math.Mod(0.3, 0.1) == 0 {
		t.Fatal("decimal multipleOf counterexample was not reproduced")
	}
	t.Logf("%d numeric cases match JSON.stringify; decimal-rational multipleOf avoids float remainder error", len(literals))
}

func TestDuplicateKeysAndNilArrays(t *testing.T) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"id":1,"id":2}`), &fields); err != nil || string(fields["id"]) != "2" {
		t.Fatalf("duplicate-key observation: %s, %v", fields["id"], err)
	}
	var nilItems []int
	data, err := json.Marshal(nilItems)
	if err != nil || string(data) != "null" {
		t.Fatalf("nil-array observation: %s, %v", data, err)
	}
	empty := make([]int, 0)
	data, err = json.Marshal(empty)
	if err != nil || string(data) != "[]" || reflect.DeepEqual(nilItems, empty) {
		t.Fatalf("empty-array observation: %s, %v", data, err)
	}
	if strings.TrimSpace(string(data)) != "[]" {
		t.Fatal("empty array changed")
	}
}
