package parser

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/mitchellh/mapstructure"
	. "github.com/smartystreets/goconvey/convey"
	"go.uber.org/zap"
)

func TestJParser(t *testing.T) {

	Convey("JParser测试套件", t, func() {

		Convey("JParser配置解码和验证", func() {
			Convey("应该成功解码和验证有效配置", func() {
				validConfigMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"tag": map[string]interface{}{
								"id": "'test-device-1'",
							},
							"field": map[string]interface{}{
								"temp": "Data['main']['temp']",
								"hum":  "Data['main']['humidity']",
							},
						},
					},
					"globalMap": map[string]interface{}{
						"location": "lab",
					},
				}
				var jC jParserConfig
				err := mapstructure.Decode(validConfigMap, &jC)
				So(err, ShouldBeNil)

				// Manually perform the checks similar to those in NewJsonParser
				So(len(jC.Points), ShouldEqual, 1)
				So(jC.Points[0].Tag["id"], ShouldEqual, "'test-device-1'")
				So(len(jC.Points[0].Field) > 0, ShouldBeTrue)

				// Simulate compilation check
				var calls []string
				for i, point := range jC.Points {
					for fieldName, expression := range point.Field {
						calls = append(calls, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
					}
					for tagName, expression := range point.Tag {
						calls = append(calls, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
					}
				}
				source := strings.Join(calls, "; ") + "; nil"
				_, compileErr := expr.Compile(source, BuildJExprOptions()...)
				So(compileErr, ShouldBeNil)
			})

			Convey("解码成功但因缺少设备配置而验证失败", func() {
				invalidConfigMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"field": map[string]interface{}{
								"temp": "Data['main']['temp']",
							},
						},
					},
				}
				var jC jParserConfig
				err := mapstructure.Decode(invalidConfigMap, &jC)
				So(err, ShouldBeNil) // Decode should work
				// Check the validation condition
				So(len(jC.Points), ShouldEqual, 1)
				So(len(jC.Points[0].Tag), ShouldEqual, 0) // 没有tag
			})

			Convey("解码成功但因缺少字段配置而验证失败", func() {
				invalidConfigMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"tag": map[string]interface{}{
								"id": "'test-device-2'",
							},
						},
					},
				}
				var jC jParserConfig
				err := mapstructure.Decode(invalidConfigMap, &jC)
				So(err, ShouldBeNil)
				// Check the validation condition
				So(len(jC.Points), ShouldEqual, 1)
				So(len(jC.Points[0].Field), ShouldEqual, 0) // 没有字段
			})

			Convey("解码成功但因表达式语法无效而编译失败", func() {
				invalidConfigMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"tag": map[string]interface{}{
								"id": "'test-device-3'",
							},
							"field": map[string]interface{}{
								"temp": "Data['main']['temp", // Syntax error
							},
						},
					},
				}
				var jC jParserConfig
				err := mapstructure.Decode(invalidConfigMap, &jC)
				So(err, ShouldBeNil)

				// Simulate compilation check
				var calls []string
				for i, point := range jC.Points {
					for fieldName, expression := range point.Field {
						calls = append(calls, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
					}
					for tagName, expression := range point.Tag {
						calls = append(calls, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
					}
				}
				source := strings.Join(calls, "; ") + "; nil"
				_, compileErr := expr.Compile(source, BuildJExprOptions()...)
				So(compileErr, ShouldNotBeNil)
				So(compileErr.Error(), ShouldContainSubstring, "literal not terminated")
			})
		})

		Convey("jParser.process方法", func() {
			// --- Setup Parsers --- Create both parser instances upfront

			// 1. Set up the main parser used for most tests
			validConfigMap := map[string]interface{}{
				"points": []map[string]interface{}{
					{
						"tag": map[string]interface{}{
							"id": "'sensor-007'",
						},
						"field": map[string]interface{}{
							"temperature": "Data != nil && 'values' in Data && Data['values'] != nil && 'temp' in Data['values'] ? Data['values']['temp'] : nil",
							"humidity":    "Data != nil && 'values' in Data && Data['values'] != nil && 'hum' in Data['values'] ? Data['values']['hum'] : nil",
							"pressure":    "Data != nil && 'pressure' in Data ? Data['pressure'] : nil",
							"loc":         "GlobalMap['loc_id']",
						},
					},
				},
				"globalMap": map[string]interface{}{
					"loc_id": "zone-A",
				},
			}
			var validJC jParserConfig
			_ = mapstructure.Decode(validConfigMap, &validJC)
			var calls []string
			for i, point := range validJC.Points {
				for fieldName, expression := range point.Field {
					calls = append(calls, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
				}
				for tagName, expression := range point.Tag {
					calls = append(calls, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
				}
			}
			source := strings.Join(calls, "; ") + "; nil"
			validProgram, errCompile1 := expr.Compile(source, BuildJExprOptions()...)
			So(errCompile1, ShouldBeNil) // Assert successful compilation

			mainParser := &JParser{
				ctx:           pkg.WithLogger(context.Background(), zap.NewNop()),
				jParserConfig: validJC,
				jEnvPool:      NewJEnvPool(validJC.GlobalMap),
				program:       validProgram,
			}
			So(mainParser.program, ShouldNotBeNil)

			// 2. Setup the parser specifically designed to handle empty JSON
			configHandlesEmptyMap := map[string]interface{}{
				"points": []map[string]interface{}{
					{
						"tag": map[string]interface{}{
							"id": "'empty-test'",
						},
						"field": map[string]interface{}{
							"exists": "'key' in Data",
						},
					},
				},
			}
			var jcEmpty jParserConfig
			_ = mapstructure.Decode(configHandlesEmptyMap, &jcEmpty)
			var callsEmpty []string
			for i, point := range jcEmpty.Points {
				for fieldName, expression := range point.Field {
					callsEmpty = append(callsEmpty, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
				}
				for tagName, expression := range point.Tag {
					callsEmpty = append(callsEmpty, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
				}
			}
			sourceEmpty := strings.Join(callsEmpty, "; ") + "; nil"
			programEmpty, errCompile2 := expr.Compile(sourceEmpty, BuildJExprOptions()...)
			So(errCompile2, ShouldBeNil) // Assert successful compilation

			emptyHandlingParser := &JParser{
				ctx:           pkg.WithLogger(context.Background(), zap.NewNop()),
				jParserConfig: jcEmpty,
				jEnvPool:      NewJEnvPool(nil),
				program:       programEmpty,
			}
			So(emptyHandlingParser.program, ShouldNotBeNil)

			// --- Test Cases --- Use the appropriate parser instance

			Convey("使用主解析器处理有效的JSON", func() {
				jsonData := []byte(`{
					"timestamp": "2023-10-27T10:00:00Z",
					"values": {
						"temp": 25.5,
						"hum": 60.1
					},
					"pressure": 1013.2,
					"status": "ok"
				}`)
				pointList, err := mainParser.process(jsonData) // Use mainParser

				So(err, ShouldBeNil)
				So(pointList, ShouldNotBeNil)
				So(len(pointList), ShouldEqual, 1)

				point := pointList[0]
				So(point.Tag["id"], ShouldEqual, "sensor-007")
				So(point.Field, ShouldResemble, map[string]interface{}{
					"temperature": 25.5,
					"humidity":    60.1,
					"pressure":    1013.2,
					"loc":         "zone-A",
				})
				_, exists := point.Field["timestamp"]
				So(exists, ShouldBeFalse)
			})

			Convey("使用主解析器处理无效的JSON语法", func() {
				invalidJsonData := []byte(`{ "values": { "temp": 25.5, "hum": 60.1 }`)
				pointList, err := mainParser.process(invalidJsonData) // Use mainParser
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "unexpected end of JSON input")
				So(pointList, ShouldBeNil)
			})

			Convey("处理缺少预期键的JSON使用主解析器", func() {
				jsonData := []byte(`{
					"timestamp": "2023-10-27T11:00:00Z",
					"values": {
						"temp": 26.0,
						"hum": 59.0
					}
				}`)
				pointList, err := mainParser.process(jsonData) // Use mainParser

				So(err, ShouldBeNil)
				So(pointList, ShouldNotBeNil)
				So(len(pointList), ShouldEqual, 1)
				point := pointList[0]
				So(point.Tag["id"], ShouldEqual, "sensor-007")
				So(point.Field["temperature"], ShouldEqual, 26.0)
				So(point.Field["humidity"], ShouldEqual, 59.0)
				So(point.Field["loc"], ShouldEqual, "zone-A")
				So(point.Field["pressure"], ShouldBeNil) // Key missing -> nil field
			})

			Convey("处理表达式结果为nil字段的JSON", func() {
				configNoMatchMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"tag": map[string]interface{}{
								"id": "'sensor-optional'",
							},
							"field": map[string]interface{}{
								"optional": "'optional_val' in Data ? Data['optional_val'] : nil",
							},
						},
					},
				}
				var jcNoMatch jParserConfig
				_ = mapstructure.Decode(configNoMatchMap, &jcNoMatch)
				var callsNoMatch []string
				for i, point := range jcNoMatch.Points {
					for fieldName, expression := range point.Field {
						callsNoMatch = append(callsNoMatch, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
					}
					for tagName, expression := range point.Tag {
						callsNoMatch = append(callsNoMatch, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
					}
				}
				sourceNoMatch := strings.Join(callsNoMatch, "; ") + "; nil"
				programNoMatch, compileErr := expr.Compile(sourceNoMatch, BuildJExprOptions()...)
				So(compileErr, ShouldBeNil)
				parserNoMatch := &JParser{
					ctx:           pkg.WithLogger(context.Background(), zap.NewNop()),
					jParserConfig: jcNoMatch,
					jEnvPool:      NewJEnvPool(nil),
					program:       programNoMatch,
				}

				jsonData := []byte(`{"val": 123}`)
				pointList, err := parserNoMatch.process(jsonData)

				So(err, ShouldBeNil)
				So(pointList, ShouldNotBeNil)
				So(len(pointList), ShouldEqual, 1) // Expect 1 point
				point := pointList[0]
				So(point.Tag["id"], ShouldEqual, "sensor-optional")
				So(point.Field, ShouldResemble, map[string]interface{}{
					"optional": nil,
				})
			})

			Convey("处理空JSON", func() {
				emptyJson := []byte(`{}`)

				Convey("使用主解析器（期望nil字段）", func() {
					pointList, err := mainParser.process(emptyJson)
					So(err, ShouldBeNil)
					So(pointList, ShouldNotBeNil)
					So(len(pointList), ShouldEqual, 1)
					point := pointList[0]
					So(point.Tag["id"], ShouldEqual, "sensor-007")
					So(point.Field, ShouldResemble, map[string]interface{}{
						"temperature": nil,
						"humidity":    nil,
						"pressure":    nil,
						"loc":         "zone-A",
					})
				})

				Convey("使用空处理解析器（期望特定字段）", func() {
					pointListEmpty, errEmpty := emptyHandlingParser.process(emptyJson) // Use emptyHandlingParser
					So(errEmpty, ShouldBeNil)
					So(len(pointListEmpty), ShouldEqual, 1)
					So(pointListEmpty[0].Tag["id"], ShouldEqual, "empty-test")
					So(pointListEmpty[0].Field, ShouldResemble, map[string]interface{}{
						"exists": false,
					})
				})
			})

			Convey("处理带有null值的JSON", func() {
				jsonData := []byte(`{ "values": { "temp": null, "hum": 55.5 } }`)
				configHandlesNullMap := map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"tag": map[string]interface{}{
								"id": "'null-test'",
							},
							"field": map[string]interface{}{
								"temp_is_null": "Data['values']['temp'] == nil",
								"temp_val":     "Data['values']['temp'] ?? -999.0",
								"humidity":     "Data['values']['hum']",
							},
						},
					},
				}
				var jcNull jParserConfig
				_ = mapstructure.Decode(configHandlesNullMap, &jcNull)
				var callsNull []string
				for i, point := range jcNull.Points {
					for fieldName, expression := range point.Field {
						callsNull = append(callsNull, fmt.Sprintf("F%d(%q, %s)", i+1, fieldName, expression))
					}
					for tagName, expression := range point.Tag {
						callsNull = append(callsNull, fmt.Sprintf("T%d(%q, %s)", i+1, tagName, expression))
					}
				}
				sourceNull := strings.Join(callsNull, "; ") + "; nil"
				programNull, compileErr := expr.Compile(sourceNull, BuildJExprOptions()...)
				So(compileErr, ShouldBeNil)
				parserHandlesNull := &JParser{
					ctx:           pkg.WithLogger(context.Background(), zap.NewNop()),
					jParserConfig: jcNull,
					jEnvPool:      NewJEnvPool(nil),
					program:       programNull,
				}
				pointListNull, errNull := parserHandlesNull.process(jsonData)

				So(errNull, ShouldBeNil)
				So(len(pointListNull), ShouldEqual, 1)
				So(pointListNull[0].Tag["id"], ShouldEqual, "null-test")
				So(pointListNull[0].Field, ShouldResemble, map[string]interface{}{
					"temp_is_null": true,
					"temp_val":     -999.0,
					"humidity":     55.5,
				})

			})

		})

	})
}
