# coder-gerrit-ssh-sync

**coder-gerrit-ssh-sync** is a tool designed to streamline the process of
configuring and synchronizing SSH credentials within
[Coder](https://coder.com/) development environments to
access [Gerrit Code Review](https://www.gerritcodereview.com/) instances.
This utility automates SSH key management, ensuring that every
Coder workspace is properly set up to clone, push, and interact with
Gerrit-hosted repositories over SSH.

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

Install the project:

```bash
git clone https://github.com/Circle-Cat/coder-gerrit-ssh-sync.git
```

After installation, navigate to the project directory and build the application:

```bash
cd coder-gerrit-ssh-sync
make build
```

## Development

### Prerequisites

- **Go** (Supports the current and two previous major Go versions, following the [Go Release Policy](https://golang.org/doc/devel/release.html#policy).)
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

## Go Modules

This assumes the use of go modules (which is the default for all Go builds
as of Go 1.13).

## Dependencies

This includes go-licenses and golangci-lint, but they are kept in the `tools`
sub-module.  If you don't want those (or their dependencies, they can be
removed.

## License

This project is licensed under the [Apache License](http://www.apache.org/licenses/LICENSE-2.0).
