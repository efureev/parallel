package parallel

import (
	"github.com/efureev/reggol"
	"sync"
	"testing"
)

func TestLogger(t *testing.T) {
	var once sync.Once
	var loggerInstance *reggol.Logger

	tests := []struct {
		name string
	}{
		{name: "LoggerInitializesOnlyOnce"},
		{name: "LoggerNotNil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			once.Do(func() {
				loggerInstance = Logger()
			})

			switch tt.name {
			case "LoggerInitializesOnlyOnce":
				l1 := Logger()
				l2 := Logger()
				if l1 != l2 {
					t.Errorf("expected logger instances to be the same, got different instances")
				}
			case "LoggerNotNil":
				if loggerInstance == nil {
					t.Errorf("Logger returned nil instance")
				}
			}
		})
	}
}

func TestCreateLogger(t *testing.T) {
	l := createLogger()
	if l == nil {
		t.Fatal("Expected a non-nil logger, got nil")
	}
}

func TestCreateTransformer(t *testing.T) {
	trans := createTransformer()
	if trans == nil {
		t.Fatal("Expected a non-nil transformer, got nil")
	}
}
