# EnvChanter

<p align="center">
   <img width="512" height="512" alt="EnvChanter_WhiteBackGround" src="https://github.com/user-attachments/assets/0392af46-75c3-4398-9790-5f2121756d02" />
</p>

EnvChanter is a lightweight command-line tool written in Go that reads parameters from AWS Systems Manager (SSM) Parameter Store or Azure Key Vault and generates a `.env` file locally. It also supports pushing local environment variables to AWS SSM, allowing development teams to securely store and manage centralized environment configuration in both directions.

## Inspiration

This project was inspired by [envilder](https://github.com/macalbert/envilder), a TypeScript-based tool. EnvChanter provides a Go implementation with cross-platform binaries.

## Features

- 🔒 **Secure secret management** - Fetch secrets directly from AWS SSM Parameter Store or Azure Key Vault
- 📤 **Push mode** - Upload local .env files to AWS SSM Parameter Store
- 🔄 **Sync mode** - Compare and update local .env files with AWS SSM values
- 📝 **Simple mapping** - Define JSON mappings between environment variables and SSM paths or Azure secret names
- 🔐 **IAM-based access control** - Use AWS IAM policies to control who can access which parameters
- ☁️ **Azure support** - Fetch secrets from Azure Key Vault using managed identities or Azure CLI credentials
- 🌍 **Multi-profile support** - Support for multiple AWS profiles and regions
- 💾 **Local .env generation** - Generate standard .env files for local development
- 🚀 **Cross-platform** - Binaries available for Linux and Windows

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/GreystoneUK/EnvChanter/releases) page.

#### Linux

```bash
# Download the binary
wget https://github.com/GreystoneUK/EnvChanter/releases/latest/download/envchanter

# Make it executable
chmod +x envchanter

# Move to your PATH (optional)
sudo mv envchanter /usr/local/bin/envchanter
```

#### Windows

Download `envchanter.exe` and add it to your PATH.

#### macOS

Download the appropriate binary for your Mac:

- For Apple Silicon (M1/M2/M3): `envchanter-macos-arm64`

```bash
# Download the binary (example for Apple Silicon)
curl -LO https://github.com/GreystoneUK/EnvChanter/releases/latest/download/envchanter-macos-arm64

# Make it executable
chmod +x envchanter-macos-arm64

# Move to your PATH (optional)
sudo mv envchanter-macos-arm64 /usr/local/bin/envchanter
```

### Build from Source

Requirements:

- Go 1.21 or later
- AWS credentials configured (via AWS CLI or environment variables)

```bash
# Clone the repository
git clone https://github.com/GreystoneUK/EnvChanter.git
cd EnvChanter

# Build the binary
go build -o envchanter .
```

## Prerequisites

### For AWS SSM

1. **AWS CLI configured** - EnvChanter uses the same credentials as AWS CLI

   ```bash
   aws configure
   ```

2. **IAM Permissions** - Your AWS user/role needs the following permissions:

   For **pull mode** (fetching from SSM):

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "ssm:GetParameter",
           "ssm:GetParameters"
         ],
         "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/app001/*"
       }
     ]
   }
   ```

   For **push mode** (uploading to SSM), add:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "ssm:GetParameter",
           "ssm:GetParameters",
           "ssm:PutParameter"
         ],
         "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/app001/*"
       }
     ]
   }
   ```

### For Azure Key Vault

1. **Azure CLI configured** - EnvChanter uses Azure DefaultAzureCredential

   ```bash
   az login
   ```

2. **Azure Permissions** - Your Azure user/managed identity needs the following permissions on the Key Vault:

   - `Key Vault Secrets User` role (for read-only access)
   - Or assign specific permissions:
     - `Get` permission on secrets

   You can assign the role using Azure CLI:

   ```bash
   az role assignment create \
     --role "Key Vault Secrets User" \
     --assignee <your-user-or-service-principal-id> \
     --scope /subscriptions/<subscription-id>/resourceGroups/<resource-group>/providers/Microsoft.KeyVault/vaults/<vault-name>
   ```

## Usage

EnvChanter supports three modes of operation:

- **Pull mode** (default): Fetch parameters from AWS SSM or Azure Key Vault and generate a local `.env` file
- **Push mode**: Upload local environment variables to AWS SSM Parameter Store (AWS only)
- **Sync mode**: Compare local `.env` with AWS SSM and update differences (AWS only)

### Command-Line Options

```bash
  -map string
        Path to JSON file mapping env vars to SSM parameter paths or Azure secret names
  -env string
        Path to .env file (for pull: output file, for push: input file) (default ".env")
  -profile string
        AWS profile to use (uses default profile if not specified)
  -region string
        AWS region to use (uses default region if not specified)
  -push
        Push mode: upload local .env to SSM (AWS only)
  -sync
        Sync mode: compare .env with SSM and update differences (AWS only)
  -force
        Force mode: update all differences without prompting (only with --sync)
  -key string
        Single environment variable name to push (only with --push)
  -value string
        Value of the single environment variable to push (only with --push)
  -ssm-path string
        SSM path for the single environment variable (only with --push)
  -azure
        Use Azure Key Vault instead of AWS SSM
  -vault-name string
        Azure Key Vault name (required with --azure)
  -version
        Show version information
  -quotes
        Quote values in the .env file output
```

### Pull Mode: Generate .env from AWS SSM

#### 1. Create Parameters in AWS SSM

First, create your parameters in AWS SSM Parameter Store:

```bash
# Using AWS CLI
aws ssm put-parameter \
  --name "/app001/test/db-password" \
  --value "your-secret-password" \
  --type "SecureString"

aws ssm put-parameter \
  --name "/app001/test/api-key" \
  --value "your-api-key" \
  --type "SecureString"
```

#### 2. Create a Parameter Mapping File

Create a JSON file (e.g., `envchanter.prod.json`) that maps environment variable names to SSM parameter paths:

```json
{
  "DB_PASSWORD": "/app001/test/db-password",
  "API_KEY": "/app001/test/api-key",
  "DATABASE_URL": "/app001/test/database-url"
}
```

#### 3. Generate Your .env File

Run EnvChanter to fetch parameters and generate your `.env` file:

```bash
envchanter --map envchanter.prod.json --env .env
```

This will create a `.env` file with the values from SSM:

```bash
API_KEY=your-api-key
DATABASE_URL=postgresql://localhost:5432/mydb
DB_PASSWORD=your-secret-password
```

### Push Mode: Upload .env to AWS SSM

EnvChanter can also push your local environment variables to AWS SSM Parameter Store.

#### Push Multiple Parameters from .env File

Create a `.env` file with your environment variables:

```bash
# .env
DB_PASSWORD=secret123
API_KEY=my-api-key
DATABASE_URL=postgresql://localhost:5432/mydb
```

Then push them to AWS SSM using your mapping file:

```bash
envchanter --push --map envchanter.prod.json --env .env
```

This will upload each variable to its corresponding SSM path defined in the mapping file.

#### Push a Single Parameter

You can also push a single parameter directly without using a mapping file:

```bash
envchanter --push --key DB_PASSWORD --value "secret123" --ssm-path "/app001/test/db-password"
```

### Examples

#### Pull Mode Examples

**Using a specific AWS profile:**

```bash
envchanter --map envchanter.prod.json --profile production
```

**Using a specific AWS region:**

```bash
envchanter --map envchanter.prod.json --region eu-west-1
```

**Custom output file:**

```bash
envchanter --map envchanter.prod.json --env .env.local
```

**Combining options:**

```bash
envchanter --map envchanter.prod.json --env .env.prod --profile production --region eu-west-1
```

#### Push Mode Examples

**Push from .env file (multiple variables):**

```bash
envchanter --push --map envchanter.prod.json --env .env
```

**Push with specific AWS profile:**

```bash
envchanter --push --map envchanter.prod.json --env .env.prod --profile production
```

**Push a single parameter:**

```bash
envchanter --push --key API_KEY --value "secret123" --ssm-path "/app001/test/api-key"
```

**Push single parameter with AWS profile:**

```bash
envchanter --push --key API_KEY --value "secret123" --ssm-path "/app001/test/api-key" --profile production
```

### Sync Mode: Compare and Update .env with AWS SSM

Sync mode allows you to compare your local `.env` file with the current values stored in AWS SSM Parameter Store and selectively update your local file with the SSM values.

#### Interactive Sync Mode

This mode displays all differences and prompts you to update each parameter individually:

```bash
envchanter --sync --map envchanter.prod.json --env .env
```

When differences are found, you'll see output like:

```
Found 2 parameter(s) with differences:

1. DB_PASSWORD
   Local:  old_password
   SSM:    new_password
   Path:   /app001/test/db-password

2. API_KEY
   Local:  old_key
   SSM:    new_key
   Path:   /app001/test/api-key

Update DB_PASSWORD (1/2)? [y]es/[n]o/[a]ll/[c]ancel: y
Update API_KEY (2/2)? [y]es/[n]o/[a]ll/[c]ancel: n

✓ Successfully updated .env with 1 parameter(s) from SSM
```

**Interactive Options:**

- `y` or `yes` - Update this parameter
- `n` or `no` - Skip this parameter
- `a` or `all` - Update this and all remaining parameters
- `c` or `cancel` - Cancel and exit without further updates

#### Force Sync Mode

Use the `--force` flag to automatically update all differing values without prompting:

```bash
envchanter --sync --force --map envchanter.prod.json --env .env
```

This is useful for automated scripts or CI/CD pipelines where you want to ensure your local `.env` is always in sync with SSM.

#### Sync Mode Examples

**Sync with specific AWS profile:**

```bash
envchanter --sync --map envchanter.prod.json --profile production
```

**Force sync with custom output file:**

```bash
envchanter --sync --force --map envchanter.prod.json --env .env.prod
```

**Sync with specific AWS region:**

```bash
envchanter --sync --map envchanter.prod.json --region us-west-2
```

**Combining sync options:**

```bash
envchanter --sync --force --map envchanter.prod.json --env .env.prod --profile production --region us-east-1
```

### Azure Key Vault Mode: Pull Secrets from Azure

EnvChanter supports fetching secrets from Azure Key Vault using the `--azure` flag.

#### 1. Create Secrets in Azure Key Vault

First, create your secrets in Azure Key Vault:

```bash
# Using Azure CLI
az keyvault secret set \
  --vault-name my-vault \
  --name db-password \
  --value "your-secret-password"

az keyvault secret set \
  --vault-name my-vault \
  --name api-key \
  --value "your-api-key"
```

#### 2. Create a Parameter Mapping File

Create a JSON file (e.g., `envchanter.azure.json`) that maps environment variable names to Azure Key Vault secret names:

```json
{
  "DB_PASSWORD": "db-password",
  "API_KEY": "api-key",
  "DATABASE_URL": "database-url"
}
```

**Note:** Azure Key Vault secret names can only contain alphanumeric characters and hyphens (no underscores or slashes like AWS SSM paths).

#### 3. Generate Your .env File from Azure

Run EnvChanter with the `--azure` flag to fetch secrets from Azure Key Vault:

```bash
envchanter --azure --vault-name my-vault --map envchanter.azure.json --env .env
```

This will create a `.env` file with the secret values from Azure Key Vault:

```bash
API_KEY=your-api-key
DATABASE_URL=postgresql://localhost:5432/mydb
DB_PASSWORD=your-secret-password
```

#### Azure Mode Examples

**Basic pull from Azure Key Vault:**

```bash
envchanter --azure --vault-name my-vault --map envchanter.azure.json
```

**Custom output file:**

```bash
envchanter --azure --vault-name my-vault --map envchanter.azure.json --env .env.local
```

**Always quote values:**

```bash
envchanter --azure --vault-name my-vault --map envchanter.azure.json --quotes
```

#### Authentication Methods

EnvChanter uses Azure's `DefaultAzureCredential`, which tries the following authentication methods in order:

1. **Environment Variables** - `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID`
2. **Managed Identity** - For Azure VMs, App Services, Functions, etc.
3. **Azure CLI** - Uses credentials from `az login`
4. **Azure PowerShell** - Uses credentials from `Connect-AzAccount`

For local development, simply run:

```bash
az login
```

For production deployments, use managed identities when running on Azure infrastructure.

## Best Practices

1. **Add .env to .gitignore** - Never commit your `.env` files to version control

   ```bash
   .env
   .env.*
   ```

2. **Use hierarchical paths** - Organize your parameters with a clear hierarchy

   ```bash
   /app001/test/database/password
   /app001/prod/database/password
   /app001/staging/api/key
   /app002/test/database/password
   /app002/prod/database/password 
   /app002/staging/api/key
   ```

3. **Use SecureString type** - Always use SecureString for sensitive values in SSM

4. **Separate mapping files** - Use different mapping files for different environments

   ```bash
   envchanter.dev.json
   envchanter.test.json
   envchanter.prod.json
   envchanter.azure.json
   ```

5. **IAM/RBAC least privilege** - Grant only the minimum necessary permissions to access parameters
   - For AWS: Use IAM policies with specific resource ARNs
   - For Azure: Use RBAC with "Key Vault Secrets User" role

6. **Azure secret naming** - Azure Key Vault secret names can only contain alphanumeric characters and hyphens
   - Use hyphens instead of underscores: `db-password` not `db_password`
   - Keep names lowercase for consistency: `api-key` not `API-KEY`

## Security

EnvChanter has undergone comprehensive security auditing and implements multiple security measures:

- 🔒 **Secure file permissions** - .env files created with 0600 permissions (owner read/write only)
- 🛡️ **Path traversal protection** - Input validation prevents directory traversal attacks
- ✅ **Input validation** - All user inputs validated against secure patterns
- 🔐 **Information disclosure prevention** - Error messages sanitized to avoid leaking sensitive data
- 🚫 **Null byte injection protection** - File paths validated against null byte attacks
- ✓ **CodeQL verified** - Passed static security analysis with zero vulnerabilities

For detailed security information, incident reporting, and best practices, see [SECURITY.md](SECURITY.md).

## License

MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
