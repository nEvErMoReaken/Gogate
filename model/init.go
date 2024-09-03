package model

import "sync"

// 确保初始化只执行一次
var once sync.Once

func Init() {
	once.Do(func() {
		// 1. 确定配置中所有用到的数据源来初始化Bucket

		// 继续初始化其他文件...
	})

}
