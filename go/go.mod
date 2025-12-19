module github.com/agentic-layer/agent-gateway-krakend

go 1.25.1

require (
	github.com/go-http-utils/headers v0.0.0-20181008091004-fed159eddc2a
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// must match the latest version in https://github.com/devopsfaith/krakend-ce/blob/v2.11.0/go.sum
replace golang.org/x/sys => golang.org/x/sys v0.39.0

replace golang.org/x/text => golang.org/x/text v0.26.0

replace golang.org/x/net => golang.org/x/net v0.41.0

replace golang.org/x/time => golang.org/x/time v0.10.0

replace golang.org/x/oauth2 => golang.org/x/oauth2 v0.27.0

replace go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.39.0

replace go.opentelemetry.io/auto/sdk => go.opentelemetry.io/auto/sdk v1.2.1

replace go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0

replace go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.39.0

replace go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.39.0

replace github.com/imdario/mergo => github.com/imdario/mergo v0.3.16

replace github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
