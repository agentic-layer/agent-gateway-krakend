module github.com/agentic-layer/agent-gateway-krakend

go 1.25.1

require (
	github.com/go-http-utils/headers v0.0.0-20181008091004-fed159eddc2a
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.11.1
	k8s.io/apimachinery v0.31.0
	k8s.io/client-go v0.31.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
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
