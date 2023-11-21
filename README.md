# configurer

`configurer` loads secret into env vars from different providers. Run `configurer l -h` to see the list of available providers.

## Install

### CLI

Setting target destination:

`curl -s https://raw.githubusercontent.com/thalesfsp/configurer/main/resources/install.sh BIN_DIR=ABSOLUTE_DIR_PATH | sh`

Setting version:

`curl -s https://raw.githubusercontent.com/thalesfsp/configurer/main/resources/install.sh VERSION=v{M.M.P} | sh`

Example:

`curl -s https://raw.githubusercontent.com/thalesfsp/configurer/main/resources/install.sh BIN_DIR=/usr/local/bin VERSION=v1.3.17 | sh`

### Programatically

Install dependency:

`go get -u github.com/thalesfsp/configurer`

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
3. Tag. Don't need to create a release, it's automatically created by CI.

## Roadmap

Check out [CHANGELOG](CHANGELOG.md).
