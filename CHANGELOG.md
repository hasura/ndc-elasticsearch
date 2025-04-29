# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Fix nil pointer when using bearer token for auth ([#88](https://github.com/hasura/ndc-elasticsearch/pull/88))

## [1.9.0]

- Add nested filtering support for nested flattened mappings ([#85](https://github.com/hasura/ndc-elasticsearch/pull/85))
- Add nested filtering support for nested mappings ([#86](https://github.com/hasura/ndc-elasticsearch/pull/86))

## [1.8.0]

- Add basic query (no operator) support for unsupported object types ([#83](https://github.com/hasura/ndc-elasticsearch/pull/83))

## [1.7.0]

- Add support for Bearer Token in credentials provider ([#78](https://github.com/hasura/ndc-elasticsearch/pull/78))

## [1.6.0]

- DDN workspace support

## [1.5.2]

- fix `_count` aggregation queries [#74](https://github.com/hasura/ndc-elasticsearch/pull/74)
- add explain to capabilities and upgrade to version v0.1.6 [#73](https://github.com/hasura/ndc-elasticsearch/pull/73)
- re-authenticate when elasticsearch returns a 401 error [#72](https://github.com/hasura/ndc-elasticsearch/pull/72)

## [1.5.1]

- Show error if ELASTICSEARCH_URL is not set when using credentials provider ([#68](https://github.com/hasura/ndc-elasticsearch/pull/68))

## [1.5.0]

- Add support for a credentials provider service ([#65](https://github.com/hasura/ndc-elasticsearch/pull/65))

## [1.4.1]

- Patch for broken release `v1.4.1`

## [1.4.0]

### Changed

- Replace deprecated scalar types Integer, Number with ranged types ([#58](https://github.com/hasura/ndc-elasticsearch/pull/58))

- Go version and dependencies update ([#51](https://github.com/hasura/ndc-elasticsearch/pull/51))

- Update to NDC Spec v0.1.6 ([#51](https://github.com/hasura/ndc-elasticsearch/pull/51))

### Added

- Implement `/query/explain` endpoint ([#57](https://github.com/hasura/ndc-elasticsearch/pull/57))

## [1.3.0]

### Added

- Add support for `search_after` in pagination ([#52](https://github.com/hasura/ndc-elasticsearch/pull/52))

### Changed

- Surface query errors in GraphQL Query response ([#52](https://github.com/hasura/ndc-elasticsearch/pull/52))

## [1.2.0]

### Changed

- Remove Native Queries from feature matrix([#54](https://github.com/hasura/ndc-elasticsearch/pull/54))

### Added

- Add index alias support ([#50](https://github.com/hasura/ndc-elasticsearch/pull/50))

## [1.1.3]

### Changed

- Set query result size to 0 if it is an aggregation query ([#46](https://github.com/hasura/ndc-elasticsearch/pull/46))

### Fixed

- Aggregation functions using subfields ([#46](https://github.com/hasura/ndc-elasticsearch/pull/46))

- Query clauses using subfields ([#44](https://github.com/hasura/ndc-elasticsearch/pull/44))

## [1.1.2]

### Changed

- Added documentation in README about limitations on queries with variables([#37](https://github.com/hasura/ndc-elasticsearch/pull/37))

### Fixed

- Added size correction to quries with variables ([#36](https://github.com/hasura/ndc-elasticsearch/pull/36))

## [1.1.1]

### Fixed

- Sorting not working for fields that enable it via subtypes ([#33](https://github.com/hasura/ndc-elasticsearch/pull/33))

## [1.1.0]

- Add a default query size of 10,000 to all queries ([#31](https://github.com/hasura/ndc-elasticsearch/pull/31))

## [1.0.3]

### Changed

- Added compound scalar types ([#27](https://github.com/hasura/ndc-elasticsearch/pull/27))
- Added support for the range operator ([#29](https://github.com/hasura/ndc-elasticsearch/pull/29))

## [1.0.2]

### Changed

- Set term as the default equality operator across scalar types
- Eliminated \_id as the default uniqueness constraint

## [1.0.1]

### Changed

- Temporarily disabled range operators while waiting on CLI updates

## [1.0.0]

### Added

- Internal plumbing for Elasticsearch Range Queries (support pending engine updates)

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
