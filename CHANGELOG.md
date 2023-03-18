# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Roadmap

- Backup strategy. When one provider fail, fallback to another.

## [1.1.29] - 2023-03-16
### Changed
- Improved non exported fields handling.

## [1.1.27] - 2023-03-16
### Changed
- The whole engine on how to handle custom tags.

## [1.1.25] - 2023-03-16
### Changed
- Fixed tests.

## [1.1.24] - 2023-03-16
### Changed
- Code improved, more consistent, better tests, and better coverage.

## [1.1.23] - 2023-03-16
### Added
- Added the ability to set default values for time.Time.
  
### Changed
- SetDefault: Improved tests, coverage, and broke down all tests into smaller ones.

## [1.1.19] - 2023-03-08
### Added
- The `id` tag. It generates a unique ID for the field if none is specified. Otherwise, it uses the specified ID. Set no ID if field is already set.

## [1.1.15] - 2023-01-17
### Changed
- Fixed bug parsing default duration.

## [1.1.14] - 2023-01-12
### Added
- Ability to use options from the CLI.

## [1.1.13] - 2023-01-01
### Changed
- Fixed lint

## [1.1.12] - 2022-12-28
### Changed
- Added the ability of the `env` tag to parse: `string`, `bool`, `int`, `float64`, `duration`, `slice` or `map`. `slice` and `map` of `string`, `bool`, `int`, or `float64`.

## [1.1.11] - 2022-12-23
### Changed
- Fixed lint

## [1.1.10] - 2022-12-23
### Changed
- Combined output only for multiple commands.

## [1.1.9] - 2022-12-23
### Changed
- Fixed lint

## [1.1.8] - 2022-12-23
### Added
- Added the capability to run one or more commands.

## [1.1.7] - 2022-12-20
### Changed
- Fixed lint

## [1.1.6] - 2022-12-20
### Added
- Added the ability to dump the loaded configuration to a file. Supported extensions are: .env, .json. .yaml, and .yml.

## [1.1.5] - 2022-12-20
### Added
- NoOp provider.

## [1.1.3] - 2022-11-10
### Added
- GetValidator returns the validator instance. Use that, for example, to add custom validators.

## [1.1.2] - 2022-11-04
### Changed
- Exposes `SetDefault` and `SetEnv`.

## [1.1.1] - 2022-11-04
### Changed
- Fixed test names.

## [1.1.0] - 2022-11-04
### Added
- `ExportToStruct` renamed to `Dump`
- `Dump` now:
    - Set values from environment variables using the `env` field tag.
    - Set default values using the `default` field tag.
    - Validating the values using the `validate` field tag.
