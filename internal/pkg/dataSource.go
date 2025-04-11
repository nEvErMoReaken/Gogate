package pkg

// Conn2ParserChan 是Connector和Parser之间传递的数据结构
type Conn2ParserChan map[string]chan []byte

// Parser2DispatcherChan 是Parser和Dispatcher之间传递的数据结构
type Parser2DispatcherChan chan *Frame2Point

// Dispatch2SinkChan 是Dispatcher和Sink之间传递的数据结构
type Dispatch2SinkChan map[string]chan *Point
