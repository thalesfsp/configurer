// Package validation provides data validation. It relies on the well
// established Go playground validator. The package is called `validation`, so
// as not to conflict with the original package. Validation applies the
// Singleton pattern, and it's safe to be retrieved at any stage of the
// application flow - it's not coupled to any other component.
package validation
