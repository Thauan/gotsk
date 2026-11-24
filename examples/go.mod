module github.com/Thauan/gotsk/examples

go 1.24.2

require github.com/Thauan/gotsk v0.0.0-00010101000000-000000000000

replace github.com/Thauan/gotsk => ..

require (
	github.com/Thauan/gotsk/middlewares/zap v0.0.0-00010101000000-000000000000
	github.com/Thauan/gotsk/store/redis v0.0.0-00010101000000-000000000000
	github.com/Thauan/gotsk/store/sqs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go-v2/config v1.33.21
	github.com/aws/aws-sdk-go-v2/service/sqs v1.38.5
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-sdk-go-v2 v1.48.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.21.3 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.53.0 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/redis/go-redis/v9 v9.7.3 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)

replace (
	github.com/Thauan/gotsk/middlewares/zap => ../middlewares/zap
	github.com/Thauan/gotsk/store/redis => ../store/redis
	github.com/Thauan/gotsk/store/sqs => ../store/sqs
)
