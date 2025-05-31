package parallel

import (
	"os"
	"testing"
)

func TestPathExists(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		setup    func() // optional setup function, can be nil
		cleanup  func() // optional cleanup function, can be nil
		expected bool
	}{
		{
			name: "existing file",
			path: "testfile.txt",
			setup: func() {
				os.WriteFile("testfile.txt", []byte("test content"), 0644)
			},
			cleanup: func() {
				os.Remove("testfile.txt")
			},
			expected: true,
		},
		{
			name:     "non-existing file",
			path:     "missingfile.txt",
			setup:    nil,
			cleanup:  nil,
			expected: false,
		},
		{
			name: "existing directory",
			path: "testdir",
			setup: func() {
				os.Mkdir("testdir", 0755)
			},
			cleanup: func() {
				os.Remove("testdir")
			},
			expected: true,
		},
		{
			name:     "non-existing directory",
			path:     "missingdir",
			setup:    nil,
			cleanup:  nil,
			expected: false,
		},
		{
			name: "no permissions",
			path: "protectedfile.txt",
			setup: func() {
				os.WriteFile("protectedfile.txt", []byte("protected"), 0000)
			},
			cleanup: func() {
				os.Chmod("protectedfile.txt", 0644)
				os.Remove("protectedfile.txt")
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			result := PathExists(tt.path)
			if result != tt.expected {
				t.Errorf("PathExists(%q) = %v; want %v", tt.path, result, tt.expected)
			}
		})
	}
}
