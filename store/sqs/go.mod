module github.com/Thauan/gotsk/store/sqs

go 1.24.2

require (
	github.com/Thauan/gotsk v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go-v2/service/sqs v1.38.5
)

require (
	github.com/aws/aws-sdk-go-v2 v1.48.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
)

replace github.com/Thauan/gotsk => ../..
