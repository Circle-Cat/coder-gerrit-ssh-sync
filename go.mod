module github.com/jingyuanliang/coder-gerrit-ssh-sync

go 1.23.0

toolchain go1.23.4

require (
	github.com/andygrunwald/go-gerrit v1.0.0
	github.com/google/go-cmp v0.5.2
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.10.0
	golang.org/x/crypto v0.37.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	golang.org/x/sys v0.32.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/andygrunwald/go-gerrit => github.com/christinak09/go-gerrit v0.0.0-20250203170103-22ab28a810d9
