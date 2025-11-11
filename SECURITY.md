# Security Policy

## Security Audit Summary

This document describes the security measures implemented in EnvChanter and provides guidance for secure usage.

## Security Audit Report (2025-11-11)

A comprehensive security audit was performed on EnvChanter. The following security enhancements have been implemented:

### 1. Secure File Permissions ✅

**Issue**: Environment files (.env) containing sensitive data were created with default permissions (0666), potentially allowing other users on the system to read secrets.

**Fix**: All .env files are now created with restrictive permissions (0600), ensuring only the owner can read and write the file.

**Impact**: Prevents unauthorized local access to secrets stored in .env files.

### 2. Path Traversal Protection ✅

**Issue**: User-supplied file paths were not validated, potentially allowing attackers to read or write files outside the intended directory using path traversal sequences like `../`.

**Fix**: Implemented `validateFilePath()` function that:
- Rejects paths containing `..` sequences
- Checks for null byte injection attempts
- Validates against empty paths

**Impact**: Prevents attackers from accessing or modifying files outside the working directory.

### 3. Input Validation ✅

**Issue**: Environment variable names and AWS SSM parameter paths were not validated, potentially allowing injection attacks or invalid data.

**Fix**: Implemented comprehensive validation:
- `validateEnvVarName()`: Ensures environment variable names follow standard naming conventions (alphanumeric and underscore only, cannot start with digit)
- `validateSSMPath()`: Validates AWS SSM paths according to AWS specifications:
  - Must start with `/`
  - Only allows alphanumeric characters, hyphens, underscores, dots, and forward slashes
  - Enforces AWS SSM maximum path length of 2048 characters
  - Prevents path traversal attempts
- `validateParameterMap()`: Validates all entries in the parameter mapping file

**Impact**: Prevents injection attacks and ensures compliance with AWS SSM specifications.

### 4. Information Disclosure Prevention ✅

**Issue**: Error messages could expose sensitive information such as internal file paths or AWS SSM parameter paths.

**Fix**: Error messages sanitized to:
- Remove SSM paths from error output
- Only show environment variable keys, not their values
- Provide generic error messages that don't leak implementation details

**Impact**: Reduces information available to potential attackers during reconnaissance.

### 5. Null Byte Injection Protection ✅

**Issue**: File paths could contain null bytes, potentially bypassing security checks in some scenarios.

**Fix**: File path validation now explicitly checks for and rejects paths containing null bytes (`\x00`).

**Impact**: Prevents null byte injection attacks.

### 6. AWS Security Best Practices ✅

**Current Implementation**:
- Uses AWS SDK v2 with secure defaults
- All parameters stored as SecureString type in AWS SSM
- Supports IAM-based access control
- Uses `WithDecryption: true` for secure parameter retrieval

**Recommendations**:
- Ensure AWS credentials are properly secured (use IAM roles when possible)
- Follow principle of least privilege for IAM permissions
- Use different AWS profiles for different environments
- Regularly rotate secrets stored in AWS SSM

## CodeQL Analysis Results

CodeQL static analysis was performed with **zero security vulnerabilities** found.

## Security Best Practices for Users

### 1. File Permissions
- Always verify that generated .env files have restrictive permissions (0600)
- Never commit .env files to version control
- Keep .env files in .gitignore

### 2. AWS IAM Permissions
Follow the principle of least privilege. For read-only operations (pull mode):
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["ssm:GetParameter", "ssm:GetParameters"],
    "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/your-app/*"
  }]
}
```

For write operations (push mode), add:
```json
{
  "Action": ["ssm:PutParameter"],
  "Resource": "arn:aws:ssm:REGION:ACCOUNT_ID:parameter/your-app/*"
}
```

### 3. Secure Storage
- Use AWS Systems Manager Parameter Store SecureString type for all secrets (default in EnvChanter)
- Enable AWS CloudTrail for audit logging of parameter access
- Consider using AWS KMS custom keys for additional encryption control

### 4. Environment Separation
- Use separate AWS accounts or regions for different environments
- Use different parameter path prefixes (e.g., `/prod/`, `/dev/`, `/staging/`)
- Maintain separate parameter mapping files for each environment

### 5. Secret Rotation
- Regularly rotate secrets stored in AWS SSM Parameter Store
- Update local .env files after rotation using sync mode
- Consider implementing automated secret rotation

### 6. Access Control
- Restrict file system access to the directory containing .env files
- Use separate AWS profiles for different team members or services
- Regularly audit AWS CloudTrail logs for unusual parameter access patterns

### 7. Network Security
- When running in production environments, ensure proper network segmentation
- Use VPC endpoints for AWS SSM API calls when possible
- Enable AWS Config to monitor SSM parameter changes

## Reporting Security Issues

If you discover a security vulnerability in EnvChanter, please report it by:

1. **DO NOT** open a public GitHub issue
2. Email the maintainers directly with details
3. Allow time for the vulnerability to be patched before public disclosure

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if available)

## Security Update Policy

- Security patches will be released as soon as possible after a vulnerability is confirmed
- Users will be notified through GitHub Security Advisories
- Critical vulnerabilities will be prioritized for immediate patching

## Changelog of Security Updates

### 2025-11-11 - Initial Security Audit
- Implemented secure file permissions (0600)
- Added path traversal protection
- Added comprehensive input validation
- Implemented information disclosure prevention
- Added null byte injection protection
- Passed CodeQL security analysis with zero vulnerabilities

## Security Testing

EnvChanter includes comprehensive security tests covering:
- Path traversal attack prevention
- File permission validation
- Input validation for all user inputs
- Null byte injection protection
- Parameter map validation

Run security tests with:
```bash
go test -v
```

## Compliance Notes

EnvChanter follows security best practices including:
- OWASP Top 10 protection measures
- Secure file handling practices
- AWS security best practices
- Input validation and sanitization
- Principle of least privilege

## Additional Resources

- [AWS Systems Manager Parameter Store Security](https://docs.aws.amazon.com/systems-manager/latest/userguide/parameter-store-security.html)
- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
