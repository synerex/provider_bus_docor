module docor-provider

go 1.13

require (
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/eclipse/paho.mqtt.golang v1.3.4
	github.com/golang/protobuf v1.5.2
	github.com/kr/pretty v0.1.0 // indirect
	github.com/shirou/gopsutil/v3 v3.21.4 // indirect
	github.com/synerex/proto_ptransit v0.0.6
	github.com/synerex/synerex_api v0.4.2
	github.com/synerex/synerex_nodeapi v0.5.4
	github.com/synerex/synerex_proto v0.1.12
	github.com/synerex/synerex_sxutil v0.6.5
	github.com/tklauser/go-sysconf v0.3.6 // indirect
	golang.org/x/net v0.0.0-20210510120150-4163338589ed // indirect
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	google.golang.org/genproto v0.0.0-20210518161634-ec7691c0a37d // indirect
	google.golang.org/grpc v1.37.1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/synerex/synerex_sxutil => ../../synerex_beta/sxutil
