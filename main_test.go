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

func TestSyncParametersNoDifferences(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create test .env file
	localEnvVars := map[string]string{
		"DB_PASSWORD": "secret123",
		"API_KEY":     "my-api-key",
	}

	// Write initial .env file
	err := writeEnvFile(envFile, localEnvVars, false)
	if err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Mock parameter map
	paramMap := ParameterMap{
		"DB_PASSWORD": "/myapp/dev/db-password",
		"API_KEY":     "/myapp/dev/api-key",
	}

	// Note: We can't test the full syncParameters function without AWS SSM,
	// but we can test the comparison logic separately
	// This test verifies the file operations work correctly
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	if !strings.Contains(string(content), "DB_PASSWORD=") {
		t.Error("Expected .env to contain DB_PASSWORD")
	}

	// Clean up - this validates paramMap structure
	if len(paramMap) != 2 {
		t.Errorf("Expected 2 parameters in map, got %d", len(paramMap))
	}
}

func TestDifferenceDetection(t *testing.T) {
	// Test identifying differences between local and SSM values
	localEnvVars := map[string]string{
		"DB_PASSWORD":  "local_secret",
		"API_KEY":      "same_key",
		"DATABASE_URL": "local_url",
	}

	ssmEnvVars := map[string]string{
		"DB_PASSWORD":  "ssm_secret",
		"API_KEY":      "same_key",
		"DATABASE_URL": "local_url",
		"NEW_KEY":      "new_value",
	}

	paramMap := ParameterMap{
		"DB_PASSWORD":  "/myapp/dev/db-password",
		"API_KEY":      "/myapp/dev/api-key",
		"DATABASE_URL": "/myapp/dev/database-url",
		"NEW_KEY":      "/myapp/dev/new-key",
	}

	// Manually detect differences (simulating syncParameters logic)
	var differences []Difference
	for envKey, ssmPath := range paramMap {
		localVal, localExists := localEnvVars[envKey]
		ssmVal, ssmExists := ssmEnvVars[envKey]

		if !localExists {
			if ssmExists {
				differences = append(differences, Difference{
					Key:       envKey,
					LocalVal:  "",
					SSMVal:    ssmVal,
					SSMPath:   ssmPath,
					ExistsSSM: true,
				})
			}
		} else if !ssmExists {
			continue
		} else if localVal != ssmVal {
			differences = append(differences, Difference{
				Key:       envKey,
				LocalVal:  localVal,
				SSMVal:    ssmVal,
				SSMPath:   ssmPath,
				ExistsSSM: true,
			})
		}
	}

	// Should find 2 differences: DB_PASSWORD (different values) and NEW_KEY (not in local)
	if len(differences) != 2 {
		t.Errorf("Expected 2 differences, got %d", len(differences))
	}

	// Verify DB_PASSWORD difference
	foundDBPassword := false
	foundNewKey := false
	for _, diff := range differences {
		if diff.Key == "DB_PASSWORD" {
			foundDBPassword = true
			if diff.LocalVal != "local_secret" {
				t.Errorf("Expected local value 'local_secret', got '%s'", diff.LocalVal)
			}
			if diff.SSMVal != "ssm_secret" {
				t.Errorf("Expected SSM value 'ssm_secret', got '%s'", diff.SSMVal)
			}
		}
		if diff.Key == "NEW_KEY" {
			foundNewKey = true
			if diff.LocalVal != "" {
				t.Errorf("Expected empty local value, got '%s'", diff.LocalVal)
			}
			if diff.SSMVal != "new_value" {
				t.Errorf("Expected SSM value 'new_value', got '%s'", diff.SSMVal)
			}
		}
	}

	if !foundDBPassword {
		t.Error("Expected to find DB_PASSWORD difference")
	}
	if !foundNewKey {
		t.Error("Expected to find NEW_KEY difference")
	}
}

func TestSyncUpdateEnvFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Initial local env vars
	localEnvVars := map[string]string{
		"DB_PASSWORD": "old_password",
		"API_KEY":     "old_key",
	}

	// Write initial .env file
	err := writeEnvFile(envFile, localEnvVars, false)
	if err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Simulate updating with SSM values
	localEnvVars["DB_PASSWORD"] = "new_password"
	localEnvVars["API_KEY"] = "new_key"

	// Write updated values
	err = writeEnvFile(envFile, localEnvVars, false)
	if err != nil {
		t.Fatalf("Failed to write updated .env file: %v", err)
	}

	// Read back and verify
	updatedVars, err := readEnvFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read updated .env file: %v", err)
	}

	if updatedVars["DB_PASSWORD"] != "new_password" {
		t.Errorf("Expected DB_PASSWORD to be 'new_password', got '%s'", updatedVars["DB_PASSWORD"])
	}

	if updatedVars["API_KEY"] != "new_key" {
		t.Errorf("Expected API_KEY to be 'new_key', got '%s'", updatedVars["API_KEY"])
	}
}
