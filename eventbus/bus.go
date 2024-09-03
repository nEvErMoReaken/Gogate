package eventbus

import (
	"sync"

	"github.com/asaskevich/EventBus"
)

// 定义一个全局的 EventBus 变量
var (
	globalEventBus EventBus.Bus
	once           sync.Once
	mu             sync.Mutex
)

// GetEventBus 返回全局唯一的 EventBus 实例
func GetEventBus() EventBus.Bus {
	once.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		globalEventBus = EventBus.New()
	})
	return globalEventBus
}
