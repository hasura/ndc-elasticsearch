# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Support for Elasticsearch Range Queries.

## [0.2.0]

### Added

- Support for native queries.
- Support ndc-spec v0.1.4 and aggregate by nested fields.

### Changed

- Configuration structure to be compatible with the latest connector version.

### Fixed

- Use static linking to resolve `glibc` version issues

## [0.1.1]

### Fixed

- Fixed the configuration directory environment variable in the CLI.
- Handled null values for nested fields in the response.

## [0.1.0]

- Initial release of the Hasura connector for Elasticsearch.
