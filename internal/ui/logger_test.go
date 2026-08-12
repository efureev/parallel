package ui

import (
	"testing"
)

func TestLogger_IsSingleton(t *testing.T) {
	l1 := Logger()
	l2 := Logger()

	if l1 == nil {
		t.Fatal("Logger returned nil instance")
	}

	if l1 != l2 {
		t.Error("expected logger instances to be the same, got different instances")
	}
}

func TestCreateLogger(t *testing.T) {
	if l := createLogger(); l == nil {
		t.Fatal("Expected a non-nil logger, got nil")
	}
}

func TestCreateTransformer(t *testing.T) {
	if trans := CreateTransformer(); trans == nil {
		t.Fatal("Expected a non-nil transformer, got nil")
	}
}
