// Copyright 2022 The configurer Authors. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

module github.com/thalesfsp/configurer

go 1.25.0

require (
	github.com/araddon/dateparse v0.0.0-20210429162001-6b43995a97de
	github.com/aws/aws-sdk-go-v2 v1.41.9
	github.com/aws/aws-sdk-go-v2/config v1.32.20
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.9
	github.com/aws/aws-sdk-go-v2/service/ssm v1.68.8
	github.com/go-playground/validator/v10 v10.30.3
	github.com/google/uuid v1.6.0
	github.com/hashicorp/vault/api v1.23.0
	github.com/iancoleman/strcase v0.3.0
	github.com/kvz/logstreamer v0.0.0-20221024075423-bf5cfbd32e39
	github.com/pelletier/go-toml v1.9.5
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	github.com/thalesfsp/concurrentloop v1.4.4
	github.com/thalesfsp/customerror v1.2.9
	github.com/thalesfsp/godotenv v1.4.2
	github.com/thalesfsp/httpclient v1.2.7
	github.com/thalesfsp/sypl/es/v2 v2.0.0
	github.com/thalesfsp/sypl/v2 v2.0.0
	github.com/thalesfsp/validation v0.0.3
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/awnumar/memcall v0.5.0 // indirect
	github.com/awnumar/memguard v0.23.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.19 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.3 // indirect
	github.com/aws/smithy-go v1.26.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.11.0 // indirect
	github.com/elastic/go-elasticsearch/v8 v8.19.6 // indirect
	github.com/elastic/go-licenser v0.4.2 // indirect
	github.com/elastic/go-sysinfo v1.15.4 // indirect
	github.com/elastic/go-windows v1.0.2 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/hpcloud/tail v1.0.0 // indirect
	github.com/jcchavezs/porto v0.7.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/sevlyar/go-daemon v0.1.6 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/sourcegraph/jsonrpc2 v0.2.1 // indirect
	github.com/thalesfsp/randomness v0.0.9 // indirect
	github.com/thalesfsp/status v1.0.22 // indirect
	go.elastic.co/apm v1.15.0 // indirect
	go.elastic.co/fastjson v1.5.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/lint v0.0.0-20241112194109-818c5a804067 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/telemetry v0.0.0-20260527142108-59979362b252 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	howett.net/plist v1.0.1 // indirect
)

require (
	github.com/fatih/color v1.19.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.2.0 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/thalesfsp/mole v1.0.2
	github.com/thalesfsp/sypl v1.19.20 // indirect
	golang.org/x/crypto v0.52.0
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)
