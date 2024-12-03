package connector

import (
	"context"
	"gateway/internal/pkg"
	. "github.com/smartystreets/goconvey/convey"
	"net"
	"testing"
	"time"
)

// 模拟 ConfigFromContext
func mockConfigFromContextUdp(url string) *pkg.Config {
	return &pkg.Config{
		Connector: pkg.ConnectorConfig{
			Type: "udp",
			Para: map[string]interface{}{
				"url":            url, // 使用自动分配端口
				"timeout":        "5s",
				"reconnectDelay": "1s",
				"whitelist":      false,
			},
		},
	}
}

func TestUdpConnector(t *testing.T) {
	Convey("测试 UdpConnector", t, func() {

		Convey("初始化配置", func() {
			ctx := pkg.WithConfig(commonCtx, mockConfigFromContextUdp("127.0.0.1:12345"))
			conn, err := NewUdpConnector(ctx)
			So(err, ShouldBeNil)
			So(conn, ShouldNotBeNil)

			udpConn, ok := conn.(*UdpConnector)
			So(ok, ShouldBeTrue)
			So(udpConn.GetType(), ShouldEqual, "message")
			So(udpConn.config.Timeout, ShouldEqual, 5*time.Second)
		})

		Convey("测试数据接收与缓冲池功能", func() {
			ctx := pkg.WithConfig(commonCtx, mockConfigFromContextUdp("127.0.0.1:12345"))
			conn, err := NewUdpConnector(ctx)
			So(err, ShouldBeNil)
			So(conn, ShouldNotBeNil)

			udpConn := conn.(*UdpConnector)

			// 创建 sourceChan 模拟数据接收
			sourceChan := make(chan pkg.DataSource, 1)

			// 使用 UDP 自动分配端口进行监听

			err = udpConn.Start(sourceChan)
			So(err, ShouldBeNil)

			// 向 UDP 发送测试数据
			addr := "127.0.0.1:12345"
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			So(err, ShouldBeNil)

			c, err := net.DialUDP("udp", nil, udpAddr)
			So(err, ShouldBeNil)
			defer c.Close()

			message := []byte("hello world")
			_, err = c.Write(message)
			So(err, ShouldBeNil)

			// 检查数据接收
			select {
			case ds := <-sourceChan:
				So(ds, ShouldNotBeNil)
				So(func() error {
					messageDataSource := ds.(*pkg.MessageDataSource)
					frame, err := messageDataSource.ReadOne()
					So(err, ShouldBeNil)
					So(string(frame), ShouldEqual, "hello world")
					return nil
				}(), ShouldBeNil)
			case <-time.After(3 * time.Second):
				So("接收消息超时", ShouldBeNil) // 强制失败提示
			}
		})

		Convey("测试 UDP 连接关闭逻辑", func() {
			ctx := pkg.WithConfig(commonCtx, mockConfigFromContextUdp("127.0.0.1:12346"))

			conn, err := NewUdpConnector(ctx)
			So(err, ShouldBeNil)
			So(conn, ShouldNotBeNil)

			udpConn := conn.(*UdpConnector)

			sourceChan := make(chan pkg.DataSource, 1)

			err = udpConn.Start(sourceChan)
			So(err, ShouldBeNil)

			// 模拟关闭连接
			time.Sleep(2 * time.Second)
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			udpConn.ctx = cancelCtx

		})
	})
}
