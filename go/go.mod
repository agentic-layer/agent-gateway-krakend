module github.com/agentic-layer/agent-gateway-krakend

go 1.25.1

require (
	github.com/atombender/go-jsonschema v0.20.0
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.10.0
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/goccy/go-yaml v1.17.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/sanity-io/litter v1.5.8 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// must match the latest version in https://github.com/devopsfaith/krakend-ce/blob/v2.10.1/go.sum
replace golang.org/x/sys => golang.org/x/sys v0.31.0

replace go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.33.0

replace go.opentelemetry.io/auto/sdk => go.opentelemetry.io/auto/sdk v1.1.0

replace go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0

replace go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.33.0

replace go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.33.0
