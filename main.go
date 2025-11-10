package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const version = "1.0.6"

const asciiArt = `
███████╗███╗   ██╗██╗   ██╗ ██████╗██╗  ██╗ █████╗ ███╗   ██╗████████╗███████╗██████╗ 
██╔════╝████╗  ██║██║   ██║██╔════╝██║  ██║██╔══██╗████╗  ██║╚══██╔══╝██╔════╝██╔══██╗
█████╗  ██╔██╗ ██║██║   ██║██║     ███████║███████║██╔██╗ ██║   ██║   █████╗  ██████╔╝
██╔══╝  ██║╚██╗██║╚██╗ ██╔╝██║     ██╔══██║██╔══██║██║╚██╗██║   ██║   ██╔══╝  ██╔══██╗
███████╗██║ ╚████║ ╚████╔╝ ╚██████╗██║  ██║██║  ██║██║ ╚████║   ██║   ███████╗██║  ██║
╚══════╝╚═╝  ╚═══╝  ╚═══╝   ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
`

type ParameterMap map[string]string

func main() {
	// Print ASCII artwork
	fmt.Print(asciiArt)

	// Define command-line flags
	mapFile := flag.String("map", "", "Path to JSON file mapping env vars to SSM parameter paths")
	envFile := flag.String("env", ".env", "Path to .env file (for pull: output file, for push: input file)")
	profile := flag.String("profile", "", "AWS profile to use")
	region := flag.String("region", "", "AWS region to use")
	showVersion := flag.Bool("version", false, "Show version information")
	push := flag.Bool("push", false, "Push mode: upload local .env to SSM")
	key := flag.String("key", "", "Single environment variable name to push (only with --push)")
	value := flag.String("value", "", "Value of the single environment variable to push (only with --push)")
	ssmPath := flag.String("ssm-path", "", "SSM path for the single environment variable (only with --push)")
	quotes := flag.Bool("quotes", false, "Always quote values in the .env file output")

	flag.Parse()

	if *showVersion {
		fmt.Printf("EnvChanter v%s\n", version)
		os.Exit(0)
	}

	// Validate flags based on mode
	if *push {
		// Push mode validation
		if *key != "" || *value != "" || *ssmPath != "" {
			// Single parameter push mode
			if *key == "" || *value == "" || *ssmPath == "" {
				fmt.Println("Error: For single parameter push, all of --key, --value, and --ssm-path are required")
				fmt.Println("\nUsage:")
				flag.PrintDefaults()
				os.Exit(1)
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
	} else {
		// Pull mode validation (existing behavior)
		if *mapFile == "" {
			fmt.Println("Error: --map flag is required")
			fmt.Println("\nUsage:")
			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	// Create AWS config
	ctx := context.Background()
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

// loadParameterMap reads the JSON mapping file
func loadParameterMap(filename string) (ParameterMap, error) {
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
				fmt.Printf("Warning: parameter not found for %s (%s), skipping.\n", envKey, ssmPath)
				continue
			}
			// For other errors, fail
			return nil, fmt.Errorf("failed to get parameter %s: %w", ssmPath, err)
		}

		if result.Parameter != nil && result.Parameter.Value != nil {
			envVars[envKey] = *result.Parameter.Value
		}
	}

	return envVars, nil
}

// writeEnvFile writes environment variables to a .env file
func writeEnvFile(filename string, envVars map[string]string, alwaysQuote bool) error {
	// Create file
	file, err := os.Create(filename)
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
		return fmt.Errorf("failed to put parameter %s: %w", ssmPath, err)
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
			return fmt.Errorf("failed to put parameter %s (%s): %w", envKey, ssmPath, err)
		}
	}

	return nil
}
