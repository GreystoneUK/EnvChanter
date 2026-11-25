# EnvChanter

EnvChanter maps local environment variables to values stored in AWS Systems Manager Parameter Store or Azure Key Vault.

## Language

**Sync**

An operation that compares mapped local environment variables with their remote values and applies selected remote values to the local `.env` file.

**Missing remote value**

A mapped value that the remote store reports as absent. Its local value, if present, is preserved during **Sync**.
