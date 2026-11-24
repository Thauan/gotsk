module github.com/Thauan/gotsk/cmd/repro

go 1.24.2

require github.com/Thauan/gotsk v0.0.0-00010101000000-000000000000

replace github.com/Thauan/gotsk => ../..

require (
	github.com/Thauan/gotsk/history/dynamodb v0.0.0-00010101000000-000000000000
	github.com/Thauan/gotsk/history/sqlite v0.0.0-00010101000000-000000000000
	github.com/Thauan/gotsk/store/sqs v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go-v2 v1.48.0
	github.com/aws/aws-sdk-go-v2/config v1.33.21
	github.com/aws/aws-sdk-go-v2/credentials v1.21.3
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.39.6
	github.com/aws/aws-sdk-go-v2/service/sqs v1.38.5
)

require (
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.48.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.53.0 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.22.0 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.34.5 // indirect
)

replace (
	github.com/Thauan/gotsk/history/dynamodb => ../../history/dynamodb
	github.com/Thauan/gotsk/history/sqlite => ../../history/sqlite
	github.com/Thauan/gotsk/store/sqs => ../../store/sqs
)
