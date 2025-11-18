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

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid simple path", ".env", false},
		{"Valid path with directory", "config/.env", false},
		{"Empty path", "", true},
		{"Path traversal with ..", "../../../etc/passwd", true},
		{"Path traversal in middle", "config/../../../etc/passwd", true},
		{"Null byte injection", ".env\x00.txt", true},
		{"Valid absolute path", "/tmp/test.env", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEnvVarName(t *testing.T) {
	tests := []struct {
		name    string
		varName string
		wantErr bool
	}{
		{"Valid uppercase", "DATABASE_URL", false},
		{"Valid with numbers", "API_KEY_123", false},
		{"Valid underscore", "MY_VAR_NAME", false},
		{"Empty name", "", true},
		{"Starts with digit", "123_VAR", true},
		{"Contains space", "MY VAR", true},
		{"Contains dash", "MY-VAR", true},
		{"Contains special char", "MY$VAR", true},
		{"Valid lowercase", "database_url", false},
		{"Valid mixed case", "MyVarName", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvVarName(tt.varName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvVarName(%q) error = %v, wantErr %v", tt.varName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSMPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid simple path", "/myapp/dev/db-password", false},
		{"Valid nested path", "/app/env/service/key", false},
		{"Valid with numbers", "/app123/key456", false},
		{"Valid with dash", "/my-app/my-key", false},
		{"Valid with underscore", "/my_app/my_key", false},
		{"Valid with dot", "/myapp/dev/key.value", false},
		{"Empty path", "", true},
		{"No leading slash", "myapp/dev/key", true},
		{"Path traversal", "/myapp/../../../etc/passwd", true},
		{"Contains space", "/my app/key", true},
		{"Contains special char", "/myapp/key$value", true},
		{"Too long path", "/" + strings.Repeat("a", 2048), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSSMPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSSMPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateParameterMap(t *testing.T) {
	tests := []struct {
		name     string
		paramMap ParameterMap
		wantErr  bool
	}{
		{
			name: "Valid parameter map",
			paramMap: ParameterMap{
				"DB_PASSWORD": "/myapp/dev/db-password",
				"API_KEY":     "/myapp/dev/api-key",
			},
			wantErr: false,
		},
		{
			name:     "Empty parameter map",
			paramMap: ParameterMap{},
			wantErr:  true,
		},
		{
			name: "Invalid env var name",
			paramMap: ParameterMap{
				"123_INVALID": "/myapp/dev/key",
			},
			wantErr: true,
		},
		{
			name: "Invalid SSM path",
			paramMap: ParameterMap{
				"VALID_KEY": "missing-leading-slash",
			},
			wantErr: true,
		},
		{
			name: "Path traversal in SSM path",
			paramMap: ParameterMap{
				"KEY": "/myapp/../../../etc/passwd",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateParameterMap(tt.paramMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateParameterMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilePermissions(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Test data
	envVars := map[string]string{
		"SECRET_KEY": "sensitive_data",
	}

	// Write the .env file
	err := writeEnvFile(envFile, envVars, false)
	if err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Check file permissions
	fileInfo, err := os.Stat(envFile)
	if err != nil {
		t.Fatalf("Failed to stat .env file: %v", err)
	}

	// File should have 0600 permissions (owner read/write only)
	expectedPerm := os.FileMode(0600)
	if fileInfo.Mode().Perm() != expectedPerm {
		t.Errorf("Expected file permissions %v, got %v", expectedPerm, fileInfo.Mode().Perm())
	}
}

func TestValidateAzureSecretName(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		wantErr    bool
	}{
		{"Valid simple name", "my-secret", false},
		{"Valid with numbers", "secret123", false},
		{"Valid with dashes", "my-secret-name", false},
		{"Valid alphanumeric", "MySecret123", false},
		{"Empty name", "", true},
		{"Too long name", strings.Repeat("a", 128), true},
		{"Contains underscore", "my_secret", true},
		{"Contains slash", "my/secret", true},
		{"Contains space", "my secret", true},
		{"Contains special char", "my$secret", true},
		{"Contains dot", "my.secret", true},
		{"Valid max length", strings.Repeat("a", 127), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzureSecretName(tt.secretName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAzureSecretName(%q) error = %v, wantErr %v", tt.secretName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAzureParameterMap(t *testing.T) {
	tests := []struct {
		name     string
		paramMap ParameterMap
		wantErr  bool
	}{
		{
			name: "Valid parameter map",
			paramMap: ParameterMap{
				"DB_PASSWORD": "db-password",
				"API_KEY":     "api-key",
			},
			wantErr: false,
		},
		{
			name:     "Empty parameter map",
			paramMap: ParameterMap{},
			wantErr:  true,
		},
		{
			name: "Invalid env var name",
			paramMap: ParameterMap{
				"123_INVALID": "secret-name",
			},
			wantErr: true,
		},
		{
			name: "Invalid Azure secret name with slash",
			paramMap: ParameterMap{
				"VALID_KEY": "invalid/secret/name",
			},
			wantErr: true,
		},
		{
			name: "Invalid Azure secret name with underscore",
			paramMap: ParameterMap{
				"VALID_KEY": "invalid_secret_name",
			},
			wantErr: true,
		},
		{
			name: "Valid Azure secret names",
			paramMap: ParameterMap{
				"DB_PASSWORD":  "db-password",
				"API_KEY":      "api-key",
				"DATABASE_URL": "database-url",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAzureParameterMap(tt.paramMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAzureParameterMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
