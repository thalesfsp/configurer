# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Roadmap

- Backup strategy. When one provider fail, fallback to another.

## [1.1.0] - 2022-11-04
### Added
- `ExportToStruct` renamed to `Dump`
- `Dump` now:
    - Set values from environment variables using the `env` field tag.
    - Set default values using the `default` field tag.
    - Validating the values using the `validate` field tag.
