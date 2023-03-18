// Package dotenv provides a `.env` provider.
package dotenv

import (
	"context"
	"testing"
	"time"

	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/configurer/util"
)

// func TestDotEnv_Load(t *testing.T) {
// 	type fields struct {
// 		Provider  *provider.Provider
// 		FilePaths []string
// 	}
// 	tests := []struct {
// 		fields      fields
// 		name        string
// 		opts        []option.KeyFunc
// 		override    bool
// 		wantErr     bool
// 		wantKey     string
// 		wantLoadErr bool
// 	}{
// 		{
// 			name: "should fail - no file paths",
// 			fields: fields{
// 				FilePaths: []string{},
// 			},
// 			wantKey: "TEST_KEY",
// 			wantErr: true,
// 		},
// 		{
// 			name: "should fail - invalid file paths",
// 			fields: fields{
// 				FilePaths: []string{"/asd/qwe/ert.env"},
// 			},
// 			wantKey:     "TEST_KEY",
// 			wantErr:     false,
// 			wantLoadErr: true,
// 		},
// 		{
// 			name: "should work",
// 			fields: fields{
// 				FilePaths: []string{"testing.env"},
// 			},
// 			wantKey: "TEST_KEY",
// 			wantErr: false,
// 		},
// 		{
// 			name: "should work with options",
// 			fields: fields{
// 				FilePaths: []string{"testing.env"},
// 			},
// 			opts: []option.KeyFunc{
// 				option.WithKeyCaser("upper"),
// 				option.WithKeyPrefixer("TESTING_DOTENV_"),
// 			},
// 			wantKey: "TESTING_DOTENV_TEST_KEY",
// 			wantErr: false,
// 		},
// 		{
// 			name: "should work with options - replacer",
// 			fields: fields{
// 				FilePaths: []string{"testing.env"},
// 			},
// 			opts: []option.KeyFunc{
// 				option.WithKeyReplacer(func(key string) string {
// 					return "TESTING123_" + key
// 				}),
// 				option.WithKeyCaser(option.Lower),
// 			},
// 			wantKey: "testing123_test_key",
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			defer os.Unsetenv(tt.wantKey)

// 			d, err := New(tt.override, tt.fields.FilePaths...)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}

// 			if d != nil {
// 				_, err := d.Load(context.Background(), tt.opts...)
// 				if (err != nil) != tt.wantLoadErr {
// 					t.Errorf("DotEnv.Load() error = %v, wantLoadErr %v", err, tt.wantLoadErr)
// 					return
// 				}

// 				if err == nil {
// 					if os.Getenv(tt.wantKey) != "TEST_VALUE" {
// 						t.Log(os.Environ())
// 						t.Errorf("Loaded config error = %v, wantErr %v", os.Getenv(tt.wantKey), "TEST_VALUE")
// 						return
// 					}
// 				}
// 			}
// 		})
// 	}
// }

type Config struct {
	CacheEnabled bool          `json:"cacheEnabled" default:"false" env:"CACHE_ENABLED"`
	CacheTTL     time.Duration `json:"cacheTTL" validate:"omitempty,gt=0" default:"1d" env:"CACHE_TTL"`

	// NOTE: For integration testing, set `environment` to `integration`.
	Environment string `json:"environment" validate:"required,oneof=integration testing development staging production" default:"development" env:"ENVIRONMENT"`

	ElasticAPMEnabled     bool   `json:"elasticAPMEnabled" default:"false" env:"ELASTIC_APM_ACTIVE"`
	ElasticAPMEnvironment string `json:"elasticAPMEnvironment" validate:"omitempty,gt=0" default:"development" env:"ELASTIC_APM_ENVIRONMENT"`
	ElasticAPMLogFile     string `json:"elasticAPMLogFile" validate:"omitempty,gt=0" env:"ELASTIC_APM_LOG_FILE"`
	ElasticAPMLogLevel    string `json:"elasticAPMLogLevel" validate:"omitempty,gt=0" env:"ELASTIC_APM_LOG_LEVEL"`
	ElasticAPMSecretToken string `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_APM_SECRET_TOKEN"`
	ElasticAPMServerURL   string `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_APM_SERVER_URL"`
	ElasticAPMServiceName string `json:"elasticAPMServiceName" validate:"omitempty,gt=0" default:"ecp-api" env:"ELASTIC_APM_SERVICE_NAME"`

	ElasticElasticSearchAPIKey   string   `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_ELASTICSEARCH_API_KEY"`
	ElasticElasticSearchCloudID  string   `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_ELASTICSEARCH_CLOUD_ID"`
	ElasticElasticSearchHosts    []string `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_ELASTICSEARCH_HOSTS"`
	ElasticElasticSearchPassword string   `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_ELASTICSEARCH_PASSWORD"`
	ElasticElasticSearchUser     string   `json:"-" validate:"omitempty,gt=0" env:"ELASTIC_ELASTICSEARCH_USER"`
	ElasticSearchEnabled         bool     `json:"elasticSearchEnabled" default:"false" env:"ELASTICSEARCH_ENABLED"`

	LoggingElasticSearchEnabled bool   `json:"loggingElasticSearchEnabled" default:"false" env:"LOGGING_ELASTICSEARCH_ENABLED"`
	LoggingFileEnabled          bool   `json:"loggingFileEnabled" default:"false" env:"LOGGING_FILE_ENABLED"`
	LoggingFilepath             string `json:"syplFilepath" validate:"omitempty,gt=0" env:"LOGGING_FILEPATH"`
	LoggingLevel                string `json:"syplLevel" validate:"required" default:"error" env:"SYPL_LEVEL"`

	LooperInterval             time.Duration `json:"interval" validate:"omitempty,gt=0" default:"5m" env:"LOOPER_INTERVAL"`
	LooperMetricsPusherEnabled bool          `json:"metricsPusherEnabled" default:"false" env:"LOOPER_METRICSPUSHER_ENABLED"`

	MongoDBEnabled  bool   `json:"mongoDBEnabled" default:"false" env:"MONGODB_ENABLED"`
	MongoDBDatabase string `json:"-" validate:"omitempty,gt=0" env:"MONGODB_DATABASE"`
	MongoDBHost     string `json:"-" validate:"omitempty,gt=0" env:"MONGODB_HOST"`

	OktaEndpoint string `json:"oktaEndpoint" validate:"omitempty,gt=0" env:"OKTA_ENDPOINT"`
	OktaEnabled  bool   `json:"oktaEnabled" default:"true" env:"OKTA_ENABLED"`

	PostgresEnabled bool   `json:"postgresEnabled" default:"false" env:"POSTGRES_ENABLED"`
	PostgresHost    string `json:"_" validate:"omitempty,gt=0" env:"POSTGRES_HOST"`

	RedisEnabled  bool   `json:"redisEnabled" default:"false" env:"REDIS_ENABLED"`
	RedisDB       int    `json:"-" validate:"omitempty,gt=0" default:"0" env:"REDIS_DATABASE"`
	RedisHost     string `json:"-" validate:"omitempty,gt=0" env:"REDIS_HOST"`
	RedisPassword string `json:"-" validate:"omitempty,gt=0" env:"REDIS_PASSWORD"`
	RedisUser     string `json:"-" validate:"omitempty,gt=0" env:"REDIS_USER"`

	// WARN: Don't change this value unless you know what you're doing.
	//
	// SEE: `resources/modules.d/golang.yml`.
	RESTPort int `json:"restAPIPort" validate:"required" default:"38123" env:"REST_API_PORT"`

	ServiceName    string `json:"serviceName" validate:"required" env:"SERVICE_NAME"`
	ServiceCompany string `json:"serviceCompany" validate:"required" env:"SERVICE_COMPANY"`

	//////
	// Timeout is the default timeout across the application.
	//
	// NOTE: Should be used anywhere a specific timeout is not needed,
	// e.g.: `TimeoutShutdown`
	//////

	TimeoutLong     time.Duration `json:"timeoutLong" validate:"omitempty,gt=0" default:"30s" env:"TIMEOUT_LONG"`
	TimeoutShort    time.Duration `json:"timeoutShort" validate:"omitempty,gt=0" default:"10s" env:"TIMEOUT_SHORT"`
	TimeoutShutdown time.Duration `json:"timeoutShutdown" validate:"omitempty,gt=0" default:"5s" env:"TIMEOUT_SHUTDOWN"`
}

func TestDotEnv_2(t *testing.T) {
	type fields struct {
		Provider  *provider.Provider
		FilePaths []string
	}
	tests := []struct {
		fields   fields
		name     string
		override bool
	}{
		{
			name: "should work",
			fields: fields{
				FilePaths: []string{"testing.env"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := New(tt.override, tt.fields.FilePaths...)
			if err != nil {
				t.Fatal(err)
			}

			s, err := d.Load(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			t.Log("HERE2", s["ELASTIC_ELASTICSEARCH_HOSTS"])

			var c Config
			if err := util.Dump(&c); err != nil {
				t.Fatal(err)
			}

			t.Log("HERE4", c.ElasticElasticSearchHosts)
		})
	}
}
