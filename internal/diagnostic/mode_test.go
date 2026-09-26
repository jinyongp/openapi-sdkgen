package diagnostic

import "testing"

func TestModeResolveDefaultsAndValidates(t *testing.T) {
	tests := []struct {
		name    string
		input   Mode
		want    Mode
		wantErr bool
	}{
		{name: "zero defaults fail fast", want: ModeFailFast},
		{name: "explicit fail fast", input: ModeFailFast, want: ModeFailFast},
		{name: "collect", input: ModeCollect, want: ModeCollect},
		{name: "invalid", input: "continue", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.input.Resolve()
			if (err != nil) != test.wantErr {
				t.Fatalf("Resolve() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("Resolve() = %q, want %q", got, test.want)
			}
		})
	}
}
