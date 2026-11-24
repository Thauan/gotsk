module github.com/Thauan/gotsk/store/redis

go 1.24.2

require (
	github.com/Thauan/gotsk v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.7.3
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace github.com/Thauan/gotsk => ../..
