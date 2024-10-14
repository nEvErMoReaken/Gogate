/*
Package parser 包含了项目的解析器部分，其通过接受一个数据源，解析到一个chan中，供策略模块使用。

parser.go -- 定义了解析器的接口

json.go -- json解析器

ioReader.go -- io.Reader解析器 （udp, tcp, file）等

*/

package parser
