package provider

import (
	"fmt"
	"os"

	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/sypl/v2"
	"github.com/thalesfsp/sypl/v2/level"
	"github.com/thalesfsp/sypl/v2/shared"
)

// anyMaxLevel reports whether any output's maxLevel equals target, or the
// SYPL_LEVEL env var names it — v1 Sypl.AnyMaxLevel semantics. Replacement
// for the v2-removed Sypl.AnyMaxLevel, per sypl's MIGRATION-V2.md.
func anyMaxLevel(l *sypl.Sypl, target level.Level) bool {
	for _, ml := range l.GetMaxLevel() { // map[outputName]level.Level
		if ml == target {
			return true
		}
	}

	return os.Getenv(shared.LevelEnvVar) == target.String()
}

// ExportToEnvVar exports the given key and value to the environment.
//
// NOTE: If override is `true`, it'll override existing environment variables!
func ExportToEnvVar(p IProvider, key string, value interface{}) (string, error) {
	fromOsEnvValue := os.Getenv(key)

	// Should export to the environment.
	finalValue := fmt.Sprintf("%v", value)

	if p.GetRawValue() {
		finalValue = fmt.Sprintf("%#v", value)
	}

	// Should allow to don't overwrite existing environment variables.
	if fromOsEnvValue != "" && !p.GetOverride() {
		finalValue = fromOsEnvValue
	}

	if err := os.Setenv(key, finalValue); err != nil {
		return "", customerror.NewFailedToError(
			fmt.Sprintf("export %s env var", key),
			customerror.WithError(err),
		)
	}

	if anyMaxLevel(p.GetLogger().Sypl, level.Debug) {
		p.GetLogger().Debuglnf("Exported key %s", key)
	} else if anyMaxLevel(p.GetLogger().Sypl, level.Trace) {
		p.GetLogger().Tracelnf("Exported key %s with value %s", key, finalValue)
	}

	return finalValue, nil
}
