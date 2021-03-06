module github.com/Sukhavati-Labs/go-miner

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/Sukhavati-Labs/skt-tendermint v0.0.1
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/go-flags v0.0.0-20150116065318-6c288d648c1c
	github.com/btcsuite/winsvc v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/fastly/go-utils v0.0.0-20180712184237-d95a45783239 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/jehiah/go-strftime v0.0.0-20171201141054-1d33003b3869 // indirect
	github.com/lestrrat/go-envload v0.0.0-20180220120943-6ed08b54a570 // indirect
	github.com/lestrrat/go-file-rotatelogs v0.0.0-20180223000712-d3151e2a480f
	github.com/lestrrat/go-strftime v0.0.0-20180220042222-ba3bf9c1d042 // indirect
	github.com/orcaman/concurrent-map v0.0.0-20190314100340-2693aad1ed75
	github.com/panjf2000/ants v1.2.0
	github.com/pkg/errors v0.8.1
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/goleveldb v1.0.1-0.20190318030020-c3a204f8e965
	github.com/tebeka/strftime v0.1.3 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/sys v0.0.0-20210119212857-b64e53b001e4
	google.golang.org/genproto v0.0.0-20201021134325-0d71844de594
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/fatih/set.v0 v0.2.1
	gopkg.in/karalabe/cookiejar.v2 v2.0.0-20150724131613-8dcd6a7f4951
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/Sukhavati-Labs/go-miner => ./

go 1.15
