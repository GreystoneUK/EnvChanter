package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var version = "dev"

const asciiArt = `
███████╗███╗   ██╗██╗   ██╗ ██████╗██╗  ██╗ █████╗ ███╗   ██╗████████╗███████╗██████╗ 
██╔════╝████╗  ██║██║   ██║██╔════╝██║  ██║██╔══██╗████╗  ██║╚══██╔══╝██╔════╝██╔══██╗
█████╗  ██╔██╗ ██║██║   ██║██║     ███████║███████║██╔██╗ ██║   ██║   █████╗  ██████╔╝
██╔══╝  ██║╚██╗██║╚██╗ ██╔╝██║     ██╔══██║██╔══██║██║╚██╗██║   ██║   ██╔══╝  ██╔══██╗
███████╗██║ ╚████║ ╚████╔╝ ╚██████╗██║  ██║██║  ██║██║ ╚████║   ██║   ███████╗██║  ██║
╚══════╝╚═╝  ╚═══╝  ╚═══╝   ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
`

type ParameterMap map[string]string

// validateFilePath validates a file path to prevent path traversal attacks
func validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty file path")
	}

	// Check for path traversal attempts
	cleanPath := strings.TrimSpace(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected")
	}

	// Check for null bytes
	if strings.Contains(cleanPath, "\x00") {
		return fmt.Errorf("null byte in file path")
	}

	return nil
}

// validateParameterMap validates the contents of a parameter map
func validateParameterMap(paramMap ParameterMap) error {
	if len(paramMap) == 0 {
		return fmt.Errorf("parameter map is empty")
	}

	for envKey, ssmPath := range paramMap {
		// Validate environment variable name
		if err := validateEnvVarName(envKey); err != nil {
			return fmt.Errorf("invalid environment variable name %q: %w", envKey, err)
		}

		// Validate SSM path
		if err := validateSSMPath(ssmPath); err != nil {
			return fmt.Errorf("invalid SSM path %q for key %q: %w", ssmPath, envKey, err)
		}
	}

	return nil
}

// validateEnvVarName validates an environment variable name
func validateEnvVarName(name string) error {
	if name == "" {
		return fmt.Errorf("empty environment variable name")
	}

	// Environment variable names should only contain alphanumeric characters and underscores
	// and should not start with a digit
	for i, char := range name {
		if i == 0 && char >= '0' && char <= '9' {
			return fmt.Errorf("environment variable name cannot start with a digit")
		}
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return fmt.Errorf("environment variable name contains invalid character: %c", char)
		}
	}

	return nil
}

// validateSSMPath validates an AWS SSM parameter path
func validateSSMPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty SSM path")
	}

	// SSM parameter names must start with /
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("SSM path must start with /")
	}

	// Check for invalid characters (AWS SSM allows alphanumeric, -, _, ., and /)
	for _, char := range path {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' ||
			char == '.' || char == '/') {
			return fmt.Errorf("SSM path contains invalid character: %c", char)
		}
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected in SSM path")
	}

	// Check length (AWS SSM has a max path length of 2048 characters)
	if len(path) > 2048 {
		return fmt.Errorf("SSM path exceeds maximum length of 2048 characters")
	}

	return nil
}

// validateAzureSecretName validates an Azure Key Vault secret name
func validateAzureSecretName(name string) error {
	if name == "" {
		return fmt.Errorf("empty secret name")
	}

	// Azure Key Vault secret names must be 1-127 characters long and contain only alphanumeric characters and hyphens
	if len(name) > 127 {
		return fmt.Errorf("secret name exceeds maximum length of 127 characters")
	}

	for _, char := range name {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '-') {
			return fmt.Errorf("secret name contains invalid character: %c (only alphanumeric and hyphens allowed)", char)
		}
	}

	return nil
}

// validateAzureParameterMap validates the contents of a parameter map for Azure Key Vault
func validateAzureParameterMap(paramMap ParameterMap) error {
	if len(paramMap) == 0 {
		return fmt.Errorf("parameter map is empty")
	}

	for envKey, secretName := range paramMap {
		// Validate environment variable name
		if err := validateEnvVarName(envKey); err != nil {
			return fmt.Errorf("invalid environment variable name %q: %w", envKey, err)
		}

		// Validate Azure secret name
		if err := validateAzureSecretName(secretName); err != nil {
			return fmt.Errorf("invalid Azure secret name %q for key %q: %w", secretName, envKey, err)
		}
	}

	return nil
}

// createAzureClient creates an Azure Key Vault client
func createAzureClient(ctx context.Context, vaultName string) (*azsecrets.Client, error) {
	// Create default Azure credential (uses managed identity, environment variables, or Azure CLI)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Construct vault URL
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)

	// Create secrets client
	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Key Vault client: %w", err)
	}

	return client, nil
}

// checkAzureAuthError checks if an error is an authentication or authorization error
func checkAzureAuthError(err error) error {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.StatusCode {
		case http.StatusUnauthorized: // 401
			return fmt.Errorf("Azure authentication failed: no valid credentials available. Please run 'az login' or configure Azure credentials")
		case http.StatusForbidden: // 403
			return fmt.Errorf("Azure authorization failed: insufficient permissions to access Key Vault. Ensure you have the required role assigned (e.g., 'Key Vault Secrets User' for read, 'Key Vault Secrets Officer' for write)")
		}
	}
	return nil
}

// fetchParametersFromAzure retrieves secret values from Azure Key Vault
func fetchParametersFromAzure(ctx context.Context, client *azsecrets.Client, paramMap ParameterMap) (map[string]string, error) {
	envVars := make(map[string]string)

	for envKey, secretName := range paramMap {
		// Get the latest version of the secret (empty version string gets latest)
		resp, err := client.GetSecret(ctx, secretName, "", nil)
		if err != nil {
			// Check for authentication/authorization errors first
			if authErr := checkAzureAuthError(err); authErr != nil {
				return nil, authErr
			}

			// Check if the error is NotFound
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
				fmt.Printf("Warning: secret not found for %s, skipping.\n", envKey)
				continue
			}

			// For other errors, fail without exposing the secret name
			return nil, fmt.Errorf("failed to get secret for %s: %w", envKey, err)
		}

		if resp.Value != nil {
			envVars[envKey] = *resp.Value
		}
	}

	return envVars, nil
}

// pushSingleParameterToAzure pushes a single parameter to Azure Key Vault
func pushSingleParameterToAzure(ctx context.Context, client *azsecrets.Client, key, value, secretName string) error {
	params := azsecrets.SetSecretParameters{
		Value: &value,
	}

	_, err := client.SetSecret(ctx, secretName, params, nil)
	if err != nil {
		// Check for authentication/authorization errors
		if authErr := checkAzureAuthError(err); authErr != nil {
			return authErr
		}
		return fmt.Errorf("failed to set secret: %w", err)
	}

	return nil
}

// pushParametersToAzure pushes multiple parameters to Azure Key Vault based on mapping
func pushParametersToAzure(ctx context.Context, client *azsecrets.Client, envVars map[string]string, paramMap ParameterMap) error {
	for envKey, secretName := range paramMap {
		value, exists := envVars[envKey]
		if !exists {
			// Skip parameters that don't exist in the .env file
			continue
		}

		params := azsecrets.SetSecretParameters{
			Value: &value,
		}

		_, err := client.SetSecret(ctx, secretName, params, nil)
		if err != nil {
			// Check for authentication/authorization errors
			if authErr := checkAzureAuthError(err); authErr != nil {
				return authErr
			}
			return fmt.Errorf("failed to set secret %s: %w", envKey, err)
		}
	}

	return nil
}

// syncParametersWithAzure compares local .env with Azure Key Vault values and updates the .env file
func syncParametersWithAzure(ctx context.Context, client *azsecrets.Client, localEnvVars map[string]string, paramMap ParameterMap, envFile string, force bool, quotes bool) error {
	// Fetch current values from Azure Key Vault
	azureEnvVars, err := fetchParametersFromAzure(ctx, client, paramMap)
	if err != nil {
		return fmt.Errorf("failed to fetch Azure Key Vault secrets: %w", err)
	}

	// Compare local and Azure values
	var differences []Difference
	for envKey, secretName := range paramMap {
		localVal, localExists := localEnvVars[envKey]
		azureVal, azureExists := azureEnvVars[envKey]

		// Check if there's a difference
		if !localExists {
			// Local doesn't have this key, but Azure does
			if azureExists {
				differences = append(differences, Difference{
					Key:       envKey,
					LocalVal:  "",
					SSMVal:    azureVal,
					SSMPath:   secretName,
					ExistsSSM: true,
				})
			}
		} else if !azureExists {
			// Local has the key but Azure doesn't - skip this
			continue
		} else if localVal != azureVal {
			// Both exist but values differ
			differences = append(differences, Difference{
				Key:       envKey,
				LocalVal:  localVal,
				SSMVal:    azureVal,
				SSMPath:   secretName,
				ExistsSSM: true,
			})
		}
	}

	// If no differences found
	if len(differences) == 0 {
		fmt.Println("✓ All values are in sync. No updates needed.")
		return nil
	}

	// Sort differences by key for consistent output
	sort.Slice(differences, func(i, j int) bool {
		return differences[i].Key < differences[j].Key
	})

	// Display differences
	fmt.Printf("\nFound %d secret(s) with differences:\n\n", len(differences))
	for i, diff := range differences {
		fmt.Printf("%d. %s\n", i+1, diff.Key)
		if diff.LocalVal == "" {
			fmt.Printf("   Local:  (not set)\n")
		} else {
			fmt.Printf("   Local:  %s\n", diff.LocalVal)
		}
		fmt.Printf("   Azure:  %s\n", diff.SSMVal)
		fmt.Printf("   Name:   %s\n\n", diff.SSMPath)
	}

	// Determine which values to update
	var toUpdate []Difference
	if force {
		// Force mode: update all differences
		toUpdate = differences
		fmt.Printf("Force mode enabled. Updating all %d secret(s)...\n", len(toUpdate))
	} else {
		// Interactive mode: prompt for each difference
		toUpdate, err = promptForUpdates(differences)
		if err != nil {
			return fmt.Errorf("error during prompting: %w", err)
		}
	}

	if len(toUpdate) == 0 {
		fmt.Println("No secrets selected for update.")
		return nil
	}

	// Update local env vars with selected Azure values
	for _, diff := range toUpdate {
		localEnvVars[diff.Key] = diff.SSMVal
	}

	// Write updated values to .env file
	err = writeEnvFile(envFile, localEnvVars, quotes)
	if err != nil {
		return fmt.Errorf("failed to write updated .env file: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated %s with %d secret(s) from Azure Key Vault\n", envFile, len(toUpdate))
	return nil
}

// loadParameterMapRaw reads the JSON mapping file without validation
func loadParameterMapRaw(filename string) (ParameterMap, error) {
	// Validate filename to prevent path traversal
	if err := validateFilePath(filename); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var paramMap ParameterMap
	err = json.Unmarshal(data, &paramMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return paramMap, nil
}

// loadParameterMap reads the JSON mapping file and validates for AWS SSM
func loadParameterMap(filename string) (ParameterMap, error) {
	paramMap, err := loadParameterMapRaw(filename)
	if err != nil {
		return nil, err
	}

	// Validate parameter map contents for AWS SSM
	if err := validateParameterMap(paramMap); err != nil {
		return nil, fmt.Errorf("invalid parameter map: %w", err)
	}

	return paramMap, nil
}

// loadAWSConfig creates an AWS config with optional profile and region
func loadAWSConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return cfg, nil
}

// fetchParameters retrieves parameter values from AWS SSM
func fetchParameters(ctx context.Context, client *ssm.Client, paramMap ParameterMap) (map[string]string, error) {
	envVars := make(map[string]string)

	for envKey, ssmPath := range paramMap {
		input := &ssm.GetParameterInput{
			Name:           &ssmPath,
			WithDecryption: boolPtr(true),
		}

		result, err := client.GetParameter(ctx, input)
		if err != nil {
			// If the error is ParameterNotFound, log a warning and continue
			if strings.Contains(err.Error(), "ParameterNotFound") {
				fmt.Printf("Warning: parameter not found for %s, skipping.\n", envKey)
				continue
			}
			// For other errors, fail without exposing the path
			return nil, fmt.Errorf("failed to get parameter for %s: %w", envKey, err)
		}

		if result.Parameter != nil && result.Parameter.Value != nil {
			envVars[envKey] = *result.Parameter.Value
		}
	}

	return envVars, nil
}

// writeEnvFile writes environment variables to a .env file
func writeEnvFile(filename string, envVars map[string]string, alwaysQuote bool) error {
	// Validate filename to prevent path traversal
	if err := validateFilePath(filename); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Create file with restrictive permissions (0600 = owner read/write only)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Sort keys for consistent output
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Write each environment variable
	for _, key := range keys {
		value := envVars[key]
		if alwaysQuote {
			value = fmt.Sprintf("\"%s\"", escapeValue(value))
		} else if needsQuoting(value) {
			value = fmt.Sprintf("\"%s\"", escapeValue(value))
		}
		_, err := fmt.Fprintf(file, "%s=%s\n", key, value)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}

// needsQuoting checks if a value needs to be quoted
func needsQuoting(value string) bool {
	return strings.ContainsAny(value, " \t\n\r\"'\\")
}

// escapeValue escapes special characters in a value
func escapeValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return value
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// readEnvFile reads a .env file and returns environment variables as a map
func readEnvFile(filename string) (map[string]string, error) {
	// Validate filename to prevent path traversal
	if err := validateFilePath(filename); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first = sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line %d: %s", i+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
				// Unescape common escape sequences
				value = strings.ReplaceAll(value, `\\`, `\`)
				value = strings.ReplaceAll(value, `\"`, `"`)
				value = strings.ReplaceAll(value, `\n`, "\n")
				value = strings.ReplaceAll(value, `\r`, "\r")
				value = strings.ReplaceAll(value, `\t`, "\t")
			}
		}

		envVars[key] = value
	}

	return envVars, nil
}

// pushSingleParameter pushes a single parameter to AWS SSM
func pushSingleParameter(ctx context.Context, client *ssm.Client, key, value, ssmPath string) error {
	input := &ssm.PutParameterInput{
		Name:      &ssmPath,
		Value:     &value,
		Type:      "SecureString",
		Overwrite: boolPtr(true),
	}

	_, err := client.PutParameter(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put parameter: %w", err)
	}

	return nil
}

// pushParameters pushes multiple parameters to AWS SSM based on mapping
func pushParameters(ctx context.Context, client *ssm.Client, envVars map[string]string, paramMap ParameterMap) error {
	for envKey, ssmPath := range paramMap {
		value, exists := envVars[envKey]
		if !exists {
			// Skip parameters that don't exist in the .env file
			continue
		}

		input := &ssm.PutParameterInput{
			Name:      &ssmPath,
			Value:     &value,
			Type:      "SecureString",
			Overwrite: boolPtr(true),
		}

		_, err := client.PutParameter(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to put parameter %s: %w", envKey, err)
		}
	}

	return nil
}

// Difference represents a parameter that differs between local and SSM
type Difference struct {
	Key       string
	LocalVal  string
	SSMVal    string
	SSMPath   string
	ExistsSSM bool
}

// syncParameters compares local .env with SSM values and updates the .env file
func syncParameters(ctx context.Context, client *ssm.Client, localEnvVars map[string]string, paramMap ParameterMap, envFile string, force bool, quotes bool) error {
	// Fetch current values from SSM
	ssmEnvVars, err := fetchParameters(ctx, client, paramMap)
	if err != nil {
		return fmt.Errorf("failed to fetch SSM parameters: %w", err)
	}

	// Compare local and SSM values
	var differences []Difference
	for envKey, ssmPath := range paramMap {
		localVal, localExists := localEnvVars[envKey]
		ssmVal, ssmExists := ssmEnvVars[envKey]

		// Check if there's a difference
		if !localExists {
			// Local doesn't have this key, but SSM does
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
			// Local has the key but SSM doesn't - skip this
			continue
		} else if localVal != ssmVal {
			// Both exist but values differ
			differences = append(differences, Difference{
				Key:       envKey,
				LocalVal:  localVal,
				SSMVal:    ssmVal,
				SSMPath:   ssmPath,
				ExistsSSM: true,
			})
		}
	}

	// If no differences found
	if len(differences) == 0 {
		fmt.Println("✓ All values are in sync. No updates needed.")
		return nil
	}

	// Sort differences by key for consistent output
	sort.Slice(differences, func(i, j int) bool {
		return differences[i].Key < differences[j].Key
	})

	// Display differences
	fmt.Printf("\nFound %d parameter(s) with differences:\n\n", len(differences))
	for i, diff := range differences {
		fmt.Printf("%d. %s\n", i+1, diff.Key)
		if diff.LocalVal == "" {
			fmt.Printf("   Local:  (not set)\n")
		} else {
			fmt.Printf("   Local:  %s\n", diff.LocalVal)
		}
		fmt.Printf("   SSM:    %s\n", diff.SSMVal)
		fmt.Printf("   Path:   %s\n\n", diff.SSMPath)
	}

	// Determine which values to update
	var toUpdate []Difference
	if force {
		// Force mode: update all differences
		toUpdate = differences
		fmt.Printf("Force mode enabled. Updating all %d parameter(s)...\n", len(toUpdate))
	} else {
		// Interactive mode: prompt for each difference
		toUpdate, err = promptForUpdates(differences)
		if err != nil {
			return fmt.Errorf("error during prompting: %w", err)
		}
	}

	if len(toUpdate) == 0 {
		fmt.Println("No parameters selected for update.")
		return nil
	}

	// Update local env vars with selected SSM values
	for _, diff := range toUpdate {
		localEnvVars[diff.Key] = diff.SSMVal
	}

	// Write updated values to .env file
	err = writeEnvFile(envFile, localEnvVars, quotes)
	if err != nil {
		return fmt.Errorf("failed to write updated .env file: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated %s with %d parameter(s) from SSM\n", envFile, len(toUpdate))
	return nil
}

// promptForUpdates prompts the user to select which parameters to update
func promptForUpdates(differences []Difference) ([]Difference, error) {
	var toUpdate []Difference

	for i, diff := range differences {
		for {
			fmt.Printf("Update %s (%d/%d)? [y]es/[n]o/[a]ll/[c]ancel: ", diff.Key, i+1, len(differences))

			var response string
			_, err := fmt.Scanln(&response)
			if err != nil {
				// Handle empty input
				response = ""
			}

			response = strings.ToLower(strings.TrimSpace(response))

			switch response {
			case "y", "yes":
				toUpdate = append(toUpdate, diff)
				goto nextDiff
			case "n", "no":
				goto nextDiff
			case "a", "all":
				// Add current and all remaining differences
				toUpdate = append(toUpdate, differences[i:]...)
				return toUpdate, nil
			case "c", "cancel":
				return toUpdate, nil
			default:
				fmt.Println("Invalid input. Please enter y(es), n(o), a(ll), or c(ancel).")
			}
		}
	nextDiff:
	}

	return toUpdate, nil
}

func main() {
	// Print ASCII artwork
	fmt.Print(asciiArt)

	// Define command-line flags
	mapFile := flag.String("map", "", "Path to JSON file mapping env vars to SSM parameter paths or Azure secret names")
	envFile := flag.String("env", ".env", "Path to .env file (for pull: output file, for push: input file)")
	profile := flag.String("profile", "", "AWS profile to use")
	region := flag.String("region", "", "AWS region to use")
	showVersion := flag.Bool("version", false, "Show version information")
	push := flag.Bool("push", false, "Push mode: upload local .env to SSM")
	sync := flag.Bool("sync", false, "Sync mode: compare .env with SSM and update differences")
	force := flag.Bool("force", false, "Force mode: update all differences without prompting (only with --sync)")
	key := flag.String("key", "", "Single environment variable name to push (only with --push)")
	value := flag.String("value", "", "Value of the single environment variable to push (only with --push)")
	ssmPath := flag.String("ssm-path", "", "SSM path for the single environment variable (only with --push and AWS)")
	secretName := flag.String("secret-name", "", "Azure Key Vault secret name for the single environment variable (only with --push and --azure)")
	quotes := flag.Bool("quotes", false, "Always quote values in the .env file output")
	azure := flag.Bool("azure", false, "Use Azure Key Vault instead of AWS SSM")
	vaultName := flag.String("vault-name", "", "Azure Key Vault name (required with --azure)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("EnvChanter %s\n", version)
		os.Exit(0)
	}

	// Validate flags based on mode
	if *push && *sync {
		fmt.Println("Error: Cannot use --push and --sync together")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Azure-specific validation
	if *azure {
		if *vaultName == "" {
			fmt.Println("Error: --vault-name is required when using --azure")
			fmt.Println("\nUsage:")
			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	if *push {
		// Push mode validation
		if *key != "" || *value != "" || *ssmPath != "" || *secretName != "" {
			// Single parameter push mode
			if *azure {
				// Azure single parameter push
				if *key == "" || *value == "" || *secretName == "" {
					fmt.Println("Error: For Azure single parameter push, all of --key, --value, and --secret-name are required")
					fmt.Println("\nUsage:")
					flag.PrintDefaults()
					os.Exit(1)
				}
			} else {
				// AWS single parameter push
				if *key == "" || *value == "" || *ssmPath == "" {
					fmt.Println("Error: For AWS single parameter push, all of --key, --value, and --ssm-path are required")
					fmt.Println("\nUsage:")
					flag.PrintDefaults()
					os.Exit(1)
				}
			}
		} else {
			// File-based push mode
			if *mapFile == "" || *envFile == "" {
				fmt.Println("Error: For file-based push, both --map and --env are required")
				fmt.Println("\nUsage:")
				flag.PrintDefaults()
				os.Exit(1)
			}
		}
	} else if *sync {
		// Sync mode validation
		if *mapFile == "" || *envFile == "" {
			fmt.Println("Error: For sync mode, both --map and --env are required")
			fmt.Println("\nUsage:")
			flag.PrintDefaults()
			os.Exit(1)
		}
	} else {
		// Pull mode validation (existing behavior)
		if *mapFile == "" {
			fmt.Println("Error: --map flag is required")
			fmt.Println("\nUsage:")
			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	ctx := context.Background()

	// Handle Azure mode
	if *azure {
		// Create Azure client
		azureClient, err := createAzureClient(ctx, *vaultName)
		if err != nil {
			fmt.Printf("Error creating Azure Key Vault client: %v\n", err)
			os.Exit(1)
		}

		if *push {
			// Azure push mode
			if *key != "" {
				// Validate key and secret name before pushing
				if err := validateEnvVarName(*key); err != nil {
					fmt.Printf("Error: invalid environment variable name: %v\n", err)
					os.Exit(1)
				}
				if err := validateAzureSecretName(*secretName); err != nil {
					fmt.Printf("Error: invalid Azure secret name: %v\n", err)
					os.Exit(1)
				}

				// Single parameter push to Azure
				err = pushSingleParameterToAzure(ctx, azureClient, *key, *value, *secretName)
				if err != nil {
					fmt.Printf("Error pushing secret: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Successfully pushed %s to Azure Key Vault secret %s\n", *key, *secretName)
			} else {
				// File-based push to Azure
				paramMap, err := loadParameterMapRaw(*mapFile)
				if err != nil {
					fmt.Printf("Error loading parameter map: %v\n", err)
					os.Exit(1)
				}

				// Validate parameter map for Azure
				if err := validateAzureParameterMap(paramMap); err != nil {
					fmt.Printf("Error: invalid parameter map: %v\n", err)
					os.Exit(1)
				}

				envVars, err := readEnvFile(*envFile)
				if err != nil {
					fmt.Printf("Error reading .env file: %v\n", err)
					os.Exit(1)
				}

				err = pushParametersToAzure(ctx, azureClient, envVars, paramMap)
				if err != nil {
					fmt.Printf("Error pushing secrets: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Successfully pushed %d secrets to Azure Key Vault\n", len(envVars))
			}
		} else if *sync {
			// Azure sync mode
			paramMap, err := loadParameterMapRaw(*mapFile)
			if err != nil {
				fmt.Printf("Error loading parameter map: %v\n", err)
				os.Exit(1)
			}

			// Validate parameter map for Azure
			if err := validateAzureParameterMap(paramMap); err != nil {
				fmt.Printf("Error: invalid parameter map: %v\n", err)
				os.Exit(1)
			}

			localEnvVars, err := readEnvFile(*envFile)
			if err != nil {
				fmt.Printf("Error reading .env file: %v\n", err)
				os.Exit(1)
			}

			err = syncParametersWithAzure(ctx, azureClient, localEnvVars, paramMap, *envFile, *force, *quotes)
			if err != nil {
				fmt.Printf("Error syncing secrets: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Azure pull mode
			paramMap, err := loadParameterMapRaw(*mapFile)
			if err != nil {
				fmt.Printf("Error loading parameter map: %v\n", err)
				os.Exit(1)
			}

			// Validate parameter map for Azure
			if err := validateAzureParameterMap(paramMap); err != nil {
				fmt.Printf("Error: invalid parameter map: %v\n", err)
				os.Exit(1)
			}

			// Fetch secrets from Azure
			envVars, err := fetchParametersFromAzure(ctx, azureClient, paramMap)
			if err != nil {
				fmt.Printf("Error fetching secrets: %v\n", err)
				os.Exit(1)
			}

			// Write .env file
			err = writeEnvFile(*envFile, envVars, *quotes)
			if err != nil {
				fmt.Printf("Error writing .env file: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully generated %s with %d secrets from Azure Key Vault\n", *envFile, len(envVars))
		}
		return
	}

	// Create AWS config
	cfg, err := loadAWSConfig(ctx, *profile, *region)
	if err != nil {
		fmt.Printf("Error loading AWS config: %v\n", err)
		os.Exit(1)
	}

	// Create SSM client
	ssmClient := ssm.NewFromConfig(cfg)

	if *push {
		// Push mode
		if *key != "" {
			// Validate key and SSM path before pushing
			if err := validateEnvVarName(*key); err != nil {
				fmt.Printf("Error: invalid environment variable name: %v\n", err)
				os.Exit(1)
			}
			if err := validateSSMPath(*ssmPath); err != nil {
				fmt.Printf("Error: invalid SSM path: %v\n", err)
				os.Exit(1)
			}

			// Single parameter push
			err = pushSingleParameter(ctx, ssmClient, *key, *value, *ssmPath)
			if err != nil {
				fmt.Printf("Error pushing parameter: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully pushed %s to %s\n", *key, *ssmPath)
		} else {
			// File-based push
			paramMap, err := loadParameterMap(*mapFile)
			if err != nil {
				fmt.Printf("Error loading parameter map: %v\n", err)
				os.Exit(1)
			}

			envVars, err := readEnvFile(*envFile)
			if err != nil {
				fmt.Printf("Error reading .env file: %v\n", err)
				os.Exit(1)
			}

			err = pushParameters(ctx, ssmClient, envVars, paramMap)
			if err != nil {
				fmt.Printf("Error pushing parameters: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully pushed %d parameters to SSM\n", len(envVars))
		}
	} else if *sync {
		// Sync mode
		paramMap, err := loadParameterMap(*mapFile)
		if err != nil {
			fmt.Printf("Error loading parameter map: %v\n", err)
			os.Exit(1)
		}

		localEnvVars, err := readEnvFile(*envFile)
		if err != nil {
			fmt.Printf("Error reading .env file: %v\n", err)
			os.Exit(1)
		}

		err = syncParameters(ctx, ssmClient, localEnvVars, paramMap, *envFile, *force, *quotes)
		if err != nil {
			fmt.Printf("Error syncing parameters: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Pull mode (existing behavior)
		paramMap, err := loadParameterMap(*mapFile)
		if err != nil {
			fmt.Printf("Error loading parameter map: %v\n", err)
			os.Exit(1)
		}

		envVars, err := fetchParameters(ctx, ssmClient, paramMap)
		if err != nil {
			fmt.Printf("Error fetching parameters: %v\n", err)
			os.Exit(1)
		}

		err = writeEnvFile(*envFile, envVars, *quotes)
		if err != nil {
			fmt.Printf("Error writing .env file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully generated %s with %d parameters\n", *envFile, len(envVars))
	}
}
