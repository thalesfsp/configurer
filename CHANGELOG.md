# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Roadmap

- Backup strategy. When one provider fail, fallback to another.
- Allows to specify multiple commands.

## [1.1.12] - 2022-12-28
### Change
- Added the ability of the `env` tag to parse: `string`, `bool`, `int`, `float64`, `duration`, `slice` or `map`. `slice` and `map` of `string`, `bool`, `int`, or `float64`.

## [1.1.11] - 2022-12-23
### Change
- Fixed lint

## [1.1.10] - 2022-12-23
### Change
- Combined output only for multiple commands.

## [1.1.9] - 2022-12-23
### Change
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
