# configurer

`configurer` loads secret into env vars from different providers.

## Install

### CLI

Install using go install:

`$ curl -s https://raw.githubusercontent.com/thalesfsp/configurer/main/resources/install.sh | sh`

### Programatically

Install dependency:

`$ go get github.com/thalesfsp/configurer`

## Usage

### CLI

See the `example` section by running `configurer l v --help` 

### Programatically

See [`dotenv/example_test.go`](dotenv/example_test.go)

### Documentation

Run `$ make doc` or check out [online](https://pkg.go.dev/github.com/thalesfsp/configurer).

## Development

Check out [CONTRIBUTION](CONTRIBUTION.md).

### Release

1. Update [CHANGELOG](CHANGELOG.md) accordingly.
2. Once changes from MR are merged.
3. Tag. Don't need to create release, it's automatically created by CI.

## Roadmap

Check out [CHANGELOG](CHANGELOG.md).
