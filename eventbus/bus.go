package eventbus

import "github.com/asaskevich/EventBus"

// 初始化全局 EventBus
var bus EventBus.Bus

// InitBus 方法初始化 EventBus
func InitBus() {
	bus = EventBus.New()
}
