package model

import (
	"fmt"
	"gw22-train-sam/config"
	"gw22-train-sam/model/strategyImpl"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

func Init(common *config.Common, proto *config.Proto, stopChan chan struct{}) {
	once.Do(func() {
		// 1. 确定配置中所有用到的数据源来初始化Bucket
		// 初始化 InfluxDB桶 的逻辑
		if common.Finally.InfluxDB.URL != "" {
			strategyImpl.NewInfluxDbStrategy(common.Finally.InfluxDB, stopChan)
		}

		// 初始化Mqtt桶 的逻辑
		if common.Finally.Mqtt.Broker != "" && common.Finally.Mqtt.Topic != "" {
			fmt.Println("Initializing MQTT with Broker:", common.Finally.Mqtt.Broker)
		}
		//
	})

}
