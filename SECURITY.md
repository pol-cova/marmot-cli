# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take the security of Marmot seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### How to Report

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to the project maintainers.

Please include the following information in your report:

- **Type of issue**: e.g., buffer overflow, SQL injection, cross-site scripting, etc.
- **Full paths of source file(s) related to the manifestation of the issue**
- **The location of the affected source code** (tag/branch/commit or direct URL)
- **Any special configuration required to reproduce the issue**
- **Step-by-step instructions to reproduce the issue**
- **Proof-of-concept or exploit code** (if possible)
- **Impact of the issue**, including how an attacker might exploit it

### What to Expect

After you submit a vulnerability report:

1. **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 5 business days
2. **Investigation**: We will investigate the issue and determine its impact
3. **Fix Development**: If confirmed, we will work on a fix
4. **Disclosure**: We will coordinate with you on the public disclosure timeline

### Security Best Practices

When using Marmot:

- **Keep your encryption keys secure**: Store backup encryption keys separately from the backup server
- **Use strong tokens**: Use strong, unique API tokens for Hub authentication
- **Secure file permissions**: Marmot sets 0600 permissions on sensitive files by default
- **Regular updates**: Keep Marmot updated to the latest version
- **Monitor access**: Review who has access to the backup server and Marmot configuration

### Security Features

Marmot includes several security features:

- **AES-256-GCM encryption**: All backups are encrypted with industry-standard encryption
- **Secure file permissions**: Configuration files, keys, and backups use 0600 permissions
- **No credential storage in logs**: Database passwords are never logged
- **Encrypted transmission**: Backups are encrypted before being transmitted to the Hub

## Acknowledgments

We will publicly acknowledge security researchers who report valid vulnerabilities (with their permission).
