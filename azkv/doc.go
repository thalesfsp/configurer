// Package azkv provides an Azure Key Vault secrets provider for configurer.
//
// The provider authenticates with Azure's default credential chain. A vault URL
// can be supplied with the AZURE_KEY_VAULT_URL environment variable or through
// the CLI. When no secret names are configured, the provider lists the vault
// and loads every secret.
//
// JSON-object secret values are exported as individual environment variables.
// Plain values use the Azure secret name as their environment key, with dashes
// converted to underscores. Writing performs the reverse conversion because
// Azure Key Vault secret names support dashes but not underscores.
package azkv
