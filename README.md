# EnvChanter

envChanter is a lightweight command-line tool written in Go that reads parameters from AWS Systems Manager (SSM) Parameter Store and generates a `.env` file locally. It also supports pushing local environment variables to SSM, allowing development teams to securely store and manage centralized environment configuration in both directions.

## Inspiration

This project was inspired by [envilder](https://github.com/macalbert/envilder), a TypeScript-based tool with similar functionality. EnvChanter provides a Go implementation with cross-platform binaries.

## Features

- 🔒 **Secure secret management** - Fetch secrets directly from AWS SSM Parameter Store
- 📤 **Push mode** - Upload local .env files to AWS SSM Parameter Store
- 📝 **Simple mapping** - Define JSON mappings between environment variables and SSM paths
- 🔐 **IAM-based access control** - Use AWS IAM policies to control who can access which parameters
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
         "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/your-app/*"
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
         "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/your-app/*"
       }
     ]
   }
   ```

## Usage

EnvChanter supports two modes of operation:

- **Pull mode** (default): Fetch parameters from AWS SSM and generate a local `.env` file
- **Push mode**: Upload local environment variables to AWS SSM Parameter Store

### Pull Mode: Generate .env from AWS SSM

#### 1. Create Parameters in AWS SSM

First, create your parameters in AWS SSM Parameter Store:

```bash
# Using AWS CLI
aws ssm put-parameter \
  --name "/myapp/dev/db-password" \
  --value "your-secret-password" \
  --type "SecureString"

aws ssm put-parameter \
  --name "/myapp/dev/api-key" \
  --value "your-api-key" \
  --type "SecureString"
```

#### 2. Create a Parameter Mapping File

Create a JSON file (e.g., `param-map.json`) that maps environment variable names to SSM parameter paths:

```json
{
  "DB_PASSWORD": "/myapp/dev/db-password",
  "API_KEY": "/myapp/dev/api-key",
  "DATABASE_URL": "/myapp/dev/database-url"
}
```

#### 3. Generate Your .env File

Run EnvChanter to fetch parameters and generate your `.env` file:

```bash
envchanter --map param-map.json --env .env
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
envchanter --push --map param-map.json --env .env
```

This will upload each variable to its corresponding SSM path defined in the mapping file.

#### Push a Single Parameter

You can also push a single parameter directly without using a mapping file:

```bash
envchanter --push --key DB_PASSWORD --value "secret123" --ssm-path "/myapp/dev/db-password"
```

### Command-Line Options

```bash
  -map string
        Path to JSON file mapping env vars to SSM parameter paths
  -env string
        Path to .env file (for pull: output file, for push: input file) (default ".env")
  -profile string
        AWS profile to use (uses default profile if not specified)
  -region string
        AWS region to use (uses default region if not specified)
  -push
        Push mode: upload local .env to SSM
  -key string
        Single environment variable name to push (only with --push)
  -value string
        Value of the single environment variable to push (only with --push)
  -ssm-path string
        SSM path for the single environment variable (only with --push)
  -version
        Show version information
```

### Examples

#### Pull Mode Examples

**Using a specific AWS profile:**

```bash
envchanter --map param-map.json --profile production
```

**Using a specific AWS region:**

```bash
envchanter --map param-map.json --region us-west-2
```

**Custom output file:**

```bash
envchanter --map param-map.json --env .env.local
```

**Combining options:**

```bash
envchanter --map param-map.json --env .env.prod --profile production --region us-east-1
```

#### Push Mode Examples

**Push from .env file (multiple variables):**

```bash
envchanter --push --map param-map.json --env .env
```

**Push with specific AWS profile:**

```bash
envchanter --push --map param-map.json --env .env.prod --profile production
```

**Push a single parameter:**

```bash
envchanter --push --key API_KEY --value "secret123" --ssm-path "/myapp/dev/api-key"
```

**Push single parameter with AWS profile:**

```bash
envchanter --push --key API_KEY --value "secret123" --ssm-path "/myapp/dev/api-key" --profile production
```

## Best Practices

1. **Add .env to .gitignore** - Never commit your `.env` files to version control

   ```bash
   .env
   .env.*
   ```

2. **Use hierarchical paths** - Organize your parameters with a clear hierarchy

   ```bash
   /myapp/dev/database/password
   /myapp/prod/database/password
   /myapp/staging/api/key
   ```

3. **Use SecureString type** - Always use SecureString for sensitive values in SSM

4. **Separate mapping files** - Use different mapping files for different environments

   ```bash
   param-map.dev.json
   param-map.staging.json
   param-map.prod.json
   ```

5. **IAM least privilege** - Grant only the minimum necessary permissions to access parameters

## License

MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
