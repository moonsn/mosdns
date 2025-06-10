module github.com/IrineSistiana/mosdns/v4

go 1.24.2

toolchain go1.24.4

require (
	github.com/AdguardTeam/dnsproxy v0.75.5
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang/snappy v0.0.4
	github.com/google/nftables v0.0.0-20221029063419-3ad45c080caa
	github.com/kardianos/service v1.2.2
	github.com/miekg/dns v1.1.65
	github.com/mitchellh/mapstructure v1.5.0
	github.com/nadoo/ipset v0.5.0
	github.com/pires/go-proxyproto v0.6.2
	github.com/prometheus/client_golang v1.19.1
	github.com/quic-go/quic-go v0.52.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/viper v1.13.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.23.0
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/net v0.39.0
	golang.org/x/sync v0.14.0
	golang.org/x/sys v0.33.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/nadoo/ipset v0.5.0 => github.com/IrineSistiana/ipset v0.5.1-0.20220703061533-6e0fc3b04c0a

require (
	github.com/AdguardTeam/golibs v0.32.9 // indirect
	github.com/ameshkov/dnscrypt/v2 v2.4.0 // indirect
	github.com/ameshkov/dnsstamps v1.0.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20250501235452-c0086092b71a // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/josharian/native v1.0.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mdlayher/netlink v1.6.2 // indirect
	github.com/mdlayher/socket v0.2.3 // indirect
	github.com/onsi/ginkgo/v2 v2.23.4 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/spf13/afero v1.9.2 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.1 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
