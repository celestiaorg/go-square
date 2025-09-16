module github.com/celestiaorg/go-square/v2

go 1.23.6

require (
	github.com/celestiaorg/nmt v0.24.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/exp v0.0.0-20231206192017-f3f8817b8deb
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract v2.3.2 // contains breaking changes
