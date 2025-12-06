package parallel

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/efureev/reggol"
)

func TestGenColors(t *testing.T) {
	tests := []struct {
		name     string
		shuffle  bool
		validate func([]reggol.TextStyle) error
	}{
		{
			name:    "NoShuffle",
			shuffle: false,
			validate: func(colors []reggol.TextStyle) error {
				expected := []reggol.TextStyle{
					reggol.ColorFgYellow,
					reggol.ColorFgRed,
					reggol.ColorFgBlue,
					reggol.ColorFgGreen,
					reggol.ColorFgCyan,
					reggol.ColorFgMagenta,
					reggol.ColorFgYellow | reggol.ColorFgBright,
					reggol.ColorFgRed | reggol.ColorFgBright,
					reggol.ColorFgBlue | reggol.ColorFgBright,
					reggol.ColorFgGreen | reggol.ColorFgBright,
					reggol.ColorFgCyan | reggol.ColorFgBright,
					reggol.ColorFgMagenta | reggol.ColorFgBright,
				}
				if !reflect.DeepEqual(colors, expected) {
					return fmt.Errorf("expected %v, got %v", expected, colors)
				}

				return nil
			},
		},
		{
			name:    "Shuffle",
			shuffle: true,
			validate: func(colors []reggol.TextStyle) error {
				if len(colors) != 12 {
					return fmt.Errorf("expected length 12, got %d", len(colors))
				}
				unique := make(map[reggol.TextStyle]bool)
				for _, clr := range colors {
					unique[clr] = true
				}
				if len(unique) != 12 {
					return fmt.Errorf("expected 12 unique colors, got %d", len(unique))
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors := GenColors(tt.shuffle)
			if err := tt.validate(colors); err != nil {
				t.Errorf("validation failed: %v", err)
			}
		})
	}
}
