package model

import (
	"gw22-train-sam/config"
	"sync"
)

// 确保初始化只执行一次
var once sync.Once

func Init(common *config.Common, proto *config.Proto, stopChan chan struct{}) {
	once.Do(func() {
		// 1. 确定配置中所有用到的数据源来初始化Bucket
		InitMapSendStrategy(common, stopChan)
		//
	})

}
