# coder-gerrit-ssh-sync

**coder-gerrit-ssh-sync** is a tool designed to streamline the process of 
configuring and synchronizing SSH credentials within 
[Coder](https://coder.ccat.dev) development environments to 
access [Gerrit Code Review](https://review.circlecat.org) instances. 
This utility automates SSH key management, ensuring that every 
Coder workspace is properly set up to clone, push, and interact with 
Gerrit-hosted repositories over SSH.

This is a skeleton project for a Go application, which captures the best build
techniques I have learned to date.  It uses a Makefile to drive the build (the
universal API to software projects) and a Dockerfile to build a docker image.

This has only been tested on Linux, and depends on Docker buildx to build.

## Features

- **Fetch Users**: Retrieve user data, including email and SSH key information, from a Coder instance using its API.
- **Connect to Gerrit**: Establish an authenticated connection to the Gerrit server using HTTP Basic Authentication.
- **Query Gerrit Accounts**: Search for matching Gerrit accounts based on user email.
- **Sync SSH Keys**: 
   - Fetch public Git SSH keys for each user from the Coder instance.
   - Add or update the SSH keys in the corresponding Gerrit accounts.
- **Logging and Auditing**: Log all operations, including successful updates and errors, for audit and troubleshooting.

The tool ensures consistency by iterating through users, verifying their SSH keys, and updating Gerrit accordingly.

## Installation

Clone the repository with the Gerrit commit-msg hook and build the application using Go.

```bash
# Clone the repository with commit-msg hook
git clone "https://review.circlecat.org/coder-gerrit-ssh-sync" && \
(cd "coder-gerrit-ssh-sync" && mkdir -p `git rev-parse --git-dir`/hooks/ && \
curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://review.circlecat.org/tools/hooks/commit-msg && \
chmod +x `git rev-parse --git-dir`/hooks/commit-msg)

# Build the application
make build
```

## Development

### Prerequisites

- **Go** (v1.13 or later)
- **Docker**

### Build

Compile the App:

```bash
make build
```

Build for All Architectures:
 
```bash
make all-build
```
### Push Image to Registry

Push Container:

```bash
make push
```

Push for All Architectures:

```bash
make all-push
```

### Manifest List

Combine all architectures into a manifest list:

```bash
make manifest-list
```

### Clean Up

Remove generated artifacts:

```bash
make clean
```

### Help

View all available targets:

```bash
make help
```

### Run Tests

Run tests and lint checks:

```bash
make test
make lint
```

The golangci-lint tool looks for configuration in `.golangci.yaml`.  If that
file is not provided, it will use its own built-in defaults.

### Docker Support

Build and run using Docker:

```bash
# Build Docker image
make container

# Build containers for all supported architectures
make all-container
```

## Customizing it

To use this, simply copy this repo and make the following changes:

Makefile:
   - change `BINS` to your binary name(s)
   - replace `cmd/myapp-*` with one directory for each of your `BINS`
   - change `REGISTRY` to the Docker registry you want to use
   - choose a strategy for `VERSION` values - git tags or manual
   - maybe change `ALL_PLATFORMS`
   - maybe change `BASE_IMAGE` (it must be a manifest-list with support for all
     platforms in `ALL_PLATFORMS`)

Dockerfile.in:
   - maybe change or remove the `USER` if you need

go.mod:
   - change module name to the one you want to use

## Go Modules

This assumes the use of go modules (which is the default for all Go builds
as of Go 1.13).

## Dependencies

This includes go-licenses and golangci-lint, but they are kept in the `tools`
sub-module.  If you don't want those (or their dependencies, they can be 
removed.

## License

This project is licensed under the [Apache License](http://www.apache.org/licenses/LICENSE-2.0).