module github.com/matheusdutra/agent-sync

go 1.24.0

toolchain go1.24.13

replace github.com/matheusdutra/token-tools => ./tools

require github.com/matheusdutra/token-tools v0.0.0-00010101000000-000000000000

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	golang.org/x/text v0.14.0 // indirect
)
