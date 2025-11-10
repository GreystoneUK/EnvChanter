package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadParameterMap(t *testing.T) {
	// Create a temporary JSON file
	tmpDir := t.TempDir()
	mapFile := filepath.Join(tmpDir, "test-map.json")

	content := `{
		"DB_PASSWORD": "/myapp/dev/db-password",
		"API_KEY": "/myapp/dev/api-key"
	}`

	err := os.WriteFile(mapFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test loading the parameter map
	paramMap, err := loadParameterMap(mapFile)
	if err != nil {
		t.Fatalf("Failed to load parameter map: %v", err)
	}

	// Verify the map contents
	if len(paramMap) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(paramMap))
	}

	if paramMap["DB_PASSWORD"] != "/myapp/dev/db-password" {
		t.Errorf("Expected /myapp/dev/db-password, got %s", paramMap["DB_PASSWORD"])
	}

	if paramMap["API_KEY"] != "/myapp/dev/api-key" {
		t.Errorf("Expected /myapp/dev/api-key, got %s", paramMap["API_KEY"])
	}
}

func TestWriteEnvFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Test data
	envVars := map[string]string{
		"DB_PASSWORD":  "secret123",
		"API_KEY":      "my-api-key",
		"DATABASE_URL": "postgresql://localhost:5432/mydb",
	}

	// Write the .env file
	err := writeEnvFile(envFile, envVars, true)
	if err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Read back the file
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	// Verify content exists
	contentStr := string(content)
	expectedVars := []string{"API_KEY=", "DATABASE_URL=", "DB_PASSWORD="}

	for _, expectedVar := range expectedVars {
		if !strings.Contains(contentStr, expectedVar) {
			t.Errorf("Expected .env to contain %s", expectedVar)
		}
	}
}

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"simple", false},
		{"has space", true},
		{"has\ttab", true},
		{"has\nline", true},
		{`has"quote`, true},
		{`has'quote`, true},
		{`has\backslash`, true},
	}

	for _, test := range tests {
		result := needsQuoting(test.value)
		if result != test.expected {
			t.Errorf("needsQuoting(%q) = %v, expected %v", test.value, result, test.expected)
		}
	}
}

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`simple`, `simple`},
		{`has\backslash`, `has\\backslash`},
		{`has"quote`, `has\"quote`},
		{"has\nline", `has\nline`},
		{"has\ttab", `has\ttab`},
	}

	for _, test := range tests {
		result := escapeValue(test.input)
		if result != test.expected {
			t.Errorf("escapeValue(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestReadEnvFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create test .env file with various formats
	content := `# This is a comment
DB_PASSWORD=secret123
API_KEY="my-api-key"
DATABASE_URL='postgresql://localhost:5432/mydb'
SIMPLE_VALUE=test

# Another comment
QUOTED_WITH_SPACES="value with spaces"
ESCAPED_QUOTES="value with \"quotes\""
EMPTY_VALUE=
`

	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Read the .env file
	envVars, err := readEnvFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	// Verify values
	tests := []struct {
		key      string
		expected string
	}{
		{"DB_PASSWORD", "secret123"},
		{"API_KEY", "my-api-key"},
		{"DATABASE_URL", "postgresql://localhost:5432/mydb"},
		{"SIMPLE_VALUE", "test"},
		{"QUOTED_WITH_SPACES", "value with spaces"},
		{"ESCAPED_QUOTES", `value with "quotes"`},
		{"EMPTY_VALUE", ""},
	}

	for _, test := range tests {
		if value, exists := envVars[test.key]; !exists {
			t.Errorf("Expected key %s to exist", test.key)
		} else if value != test.expected {
			t.Errorf("For key %s, expected %q, got %q", test.key, test.expected, value)
		}
	}
}

func TestReadEnvFileInvalidFormat(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create invalid .env file
	content := `VALID_KEY=value
INVALID_LINE_WITHOUT_EQUALS
ANOTHER_VALID=test`

	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Attempt to read the .env file
	_, err = readEnvFile(envFile)
	if err == nil {
		t.Error("Expected error for invalid .env format, got nil")
	}
}
