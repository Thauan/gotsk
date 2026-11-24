module github.com/Thauan/gotsk/middlewares/zap

go 1.24.2

require (
	github.com/Thauan/gotsk v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.27.0
)

require go.uber.org/multierr v1.10.0 // indirect

replace github.com/Thauan/gotsk => ../..
