module gateway

go 1.23.4

require (
	github.com/apache/iotdb-client-go v1.3.2 // iotdb连接sdk
	github.com/eclipse/paho.mqtt.golang v1.5.0 //  mqtt client
	github.com/google/uuid v1.6.0 // indirect; gen uuid
	github.com/influxdata/influxdb-client-go/v2 v2.14.0 // influxdb连接sdk
	github.com/mitchellh/mapstructure v1.5.0 // map转struct
	github.com/mochi-co/mqtt v1.3.2 // mock mqtt broker
	github.com/prometheus/client_golang v1.20.5 // prometheus 相关
	github.com/smartystreets/goconvey v1.8.1 // 单测
	github.com/spf13/cobra v1.8.1 // cli程序
	github.com/spf13/viper v1.19.0 // 配置读取
	github.com/stretchr/testify v1.10.0 // 其他测试
	github.com/traefik/yaegi v0.16.1 // 动态脚本解析
	go.uber.org/zap v1.27.0 // 日志
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // 日志切割
)

require github.com/shengyanli1982/law v0.1.17

require (
	github.com/apache/thrift v0.21.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/influxdata/line-protocol v0.0.0-20210922203350-b1ad95c89adf // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/magiconair/properties v1.8.9 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.61.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sagikazarmark/locafero v0.6.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20241204233417-43b7b7cde48d // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
