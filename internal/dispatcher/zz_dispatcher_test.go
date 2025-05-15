package dispatcher

import (
	"gateway/internal/pkg"
	"testing"
	"time"

	// "fmt" // Placeholder for fmt if needed later

	. "github.com/smartystreets/goconvey/convey"
)

// Mock strategy configs, reflecting that Strategy field in Handler is []pkg.StrategyConfig
var (
	strategyAll = pkg.StrategyConfig{
		Type:   "all",
		Filter: []string{"true"}, // Matches everything
	}
	strategyTypeA = pkg.StrategyConfig{
		Type:   "typeA",
		Filter: []string{`Tag.type == "A"`},
	}
	strategyTypeB = pkg.StrategyConfig{
		Type:   "typeB",
		Filter: []string{`Tag.type == "B"`},
	}
	strategyComplex = pkg.StrategyConfig{
		Type:   "complex",
		Filter: []string{`Tag.value > 10`, `Tag.status == "active"`},
	}
	strategyInvalidFilter = pkg.StrategyConfig{
		Type:   "invalid",
		Filter: []string{"this is not a valid expression !!!"},
	}
)

func TestNewHandler(t *testing.T) {
	Convey("Testing NewHandler", t, func() {
		Convey("Given a set of valid strategy configurations", func() {
			configs := []pkg.StrategyConfig{strategyAll, strategyTypeA}
			Convey("When NewHandler is called", func() {
				handler, err := NewHandler(configs)
				Convey("Then it should return a valid handler and no error", func() {
					So(err, ShouldBeNil)
					So(handler, ShouldNotBeNil)
					So(len(handler.Strategy), ShouldEqual, len(configs))
					So(handler.Strategy[0].Type, ShouldEqual, strategyAll.Type)
					So(handler.Strategy[1].Type, ShouldEqual, strategyTypeA.Type)
					So(len(handler.StrategyFilterList), ShouldEqual, len(configs))
					So(handler.StrategyFilterList[strategyAll.Type], ShouldNotBeNil)
					So(handler.StrategyFilterList[strategyTypeA.Type], ShouldNotBeNil)
				})
			})
		})

		Convey("Given a strategy configuration with an invalid filter", func() {
			configs := []pkg.StrategyConfig{strategyInvalidFilter}
			Convey("When NewHandler is called", func() {
				handler, err := NewHandler(configs)
				Convey("Then it should return an error and a nil handler", func() {
					So(err, ShouldNotBeNil)
					So(handler, ShouldBeNil)
					So(err.Error(), ShouldContainSubstring, "编译策略过滤表达式失败")
				})
			})
		})

		Convey("Given an empty set of strategy configurations", func() {
			configs := []pkg.StrategyConfig{}
			Convey("When NewHandler is called", func() {
				handler, err := NewHandler(configs)
				Convey("Then it should return a valid handler with no strategies and no error", func() {
					So(err, ShouldBeNil)
					So(handler, ShouldNotBeNil)
					So(len(handler.Strategy), ShouldEqual, 0)
					So(len(handler.StrategyFilterList), ShouldEqual, 0)
				})
			})
		})
	})
}

func TestHandlerAddPoint(t *testing.T) {
	Convey("Testing handler.AddPoint", t, func() {
		baseTime := time.Now().Round(time.Millisecond)
		baseFrameId := "frame-addpoint-123"

		Convey("Given a handler with multiple strategies", func() {
			configs := []pkg.StrategyConfig{strategyTypeA, strategyTypeB, strategyComplex}
			handler, err := NewHandler(configs)
			So(err, ShouldBeNil)
			So(handler, ShouldNotBeNil)

			handler.LatestTs = baseTime
			handler.LatestFrameId = baseFrameId

			readyPointPackage := make(map[string]*pkg.PointPackage)

			Convey("When a point matching only strategyTypeA is added", func() {
				pointA := pkg.PointPoolInstance.Get()
				pointA.Tag = map[string]any{"type": "A", "value": 5, "status": "inactive"}
				pointA.Field = map[string]any{"temp": 25.5}
				defer pkg.PointPoolInstance.Put(pointA) // Defer cleanup of original point

				errAdd := handler.AddPoint(pointA, readyPointPackage)

				// Check if the package and point exist before trying to access them
				pkgA, pkgAExists := readyPointPackage[strategyTypeA.Type]
				var clonedPointFromPkgA *pkg.Point
				if pkgAExists && len(pkgA.Points) > 0 {
					clonedPointFromPkgA = pkgA.Points[0]
					defer pkg.PointPoolInstance.Put(clonedPointFromPkgA) // Clean up the clone
				}

				Convey("Then it should be added only to strategyTypeA's package", func() {
					So(errAdd, ShouldBeNil)
					So(len(readyPointPackage), ShouldEqual, 1)
					So(readyPointPackage[strategyTypeA.Type], ShouldNotBeNil)
					So(len(readyPointPackage[strategyTypeA.Type].Points), ShouldEqual, 1)
					So(readyPointPackage[strategyTypeA.Type].Points[0].Tag["type"], ShouldEqual, "A")
					So(readyPointPackage[strategyTypeA.Type].Ts.Equal(baseTime), ShouldBeTrue)
					So(readyPointPackage[strategyTypeA.Type].FrameId, ShouldEqual, baseFrameId)

					So(readyPointPackage[strategyTypeA.Type].Points[0], ShouldNotPointTo, pointA)
					So(readyPointPackage[strategyTypeA.Type].Points[0].Tag, ShouldResemble, pointA.Tag)
					So(readyPointPackage[strategyTypeA.Type].Points[0].Field, ShouldResemble, pointA.Field)
				})
			})

			Convey("When a point matching strategyTypeB and strategyComplex is added", func() {
				readyPointPackage = make(map[string]*pkg.PointPackage) // Reset for this scenario
				pointBComplex := pkg.PointPoolInstance.Get()
				pointBComplex.Tag = map[string]any{"type": "B", "value": 20, "status": "active"}
				pointBComplex.Field = map[string]any{"humidity": 60}
				defer pkg.PointPoolInstance.Put(pointBComplex) // Defer cleanup of original point

				errAdd := handler.AddPoint(pointBComplex, readyPointPackage)

				var clonedPointFromPkgB *pkg.Point
				pkgB, pkgBExists := readyPointPackage[strategyTypeB.Type]
				if pkgBExists && len(pkgB.Points) > 0 {
					clonedPointFromPkgB = pkgB.Points[0]
					// Cloned point is shared, defer its cleanup once
					// No need to defer Put for clonedPointFromPkgComplex as it's the same point
					defer pkg.PointPoolInstance.Put(clonedPointFromPkgB)
				}

				Convey("Then it should be added to both packages, sharing the same cloned point", func() {
					So(errAdd, ShouldBeNil)
					So(len(readyPointPackage), ShouldEqual, 2)
					So(readyPointPackage[strategyTypeB.Type], ShouldNotBeNil)
					So(readyPointPackage[strategyComplex.Type], ShouldNotBeNil)

					So(len(readyPointPackage[strategyTypeB.Type].Points), ShouldEqual, 1)
					So(len(readyPointPackage[strategyComplex.Type].Points), ShouldEqual, 1)

					So(readyPointPackage[strategyTypeB.Type].Points[0], ShouldPointTo, readyPointPackage[strategyComplex.Type].Points[0])
					So(readyPointPackage[strategyTypeB.Type].Points[0], ShouldNotPointTo, pointBComplex)
					So(readyPointPackage[strategyTypeB.Type].Points[0].Tag, ShouldResemble, pointBComplex.Tag)

					So(readyPointPackage[strategyTypeB.Type].Ts.Equal(baseTime), ShouldBeTrue)
					So(readyPointPackage[strategyTypeB.Type].FrameId, ShouldEqual, baseFrameId)
					So(readyPointPackage[strategyComplex.Type].Ts.Equal(baseTime), ShouldBeTrue)
					So(readyPointPackage[strategyComplex.Type].FrameId, ShouldEqual, baseFrameId)
				})
			})

			Convey("When a point matching no strategies is added", func() {
				readyPointPackage = make(map[string]*pkg.PointPackage) // Reset
				pointNone := pkg.PointPoolInstance.Get()
				pointNone.Tag = map[string]any{"type": "C", "value": 5, "status": "inactive"}
				defer pkg.PointPoolInstance.Put(pointNone)

				errAdd := handler.AddPoint(pointNone, readyPointPackage)
				Convey("Then readyPointPackage should remain empty", func() {
					So(errAdd, ShouldBeNil)
					So(len(readyPointPackage), ShouldEqual, 0)
				})
			})
		})
	})
}

func TestHandlerDispatch(t *testing.T) {
	Convey("Testing handler.Dispatch", t, func() {
		baseTime := time.Now().Round(time.Millisecond)
		baseFrameId := "dispatch-frame-001"

		Convey("Given a handler with TypeA and TypeB strategies", func() {
			configs := []pkg.StrategyConfig{strategyTypeA, strategyTypeB}
			handler, err := NewHandler(configs)
			So(err, ShouldBeNil)

			pointA1 := pkg.PointPoolInstance.Get()
			pointA1.Tag = map[string]any{"type": "A", "id": "A1"}
			pointA1.Field = map[string]any{"val": 1}

			pointB1 := pkg.PointPoolInstance.Get()
			pointB1.Tag = map[string]any{"type": "B", "id": "B1"}
			pointB1.Field = map[string]any{"val": 2}

			pointA2 := pkg.PointPoolInstance.Get()
			pointA2.Tag = map[string]any{"type": "A", "id": "A2"}
			pointA2.Field = map[string]any{"val": 3}

			pointC1 := pkg.PointPoolInstance.Get()
			pointC1.Tag = map[string]any{"type": "C", "id": "C1"}
			pointC1.Field = map[string]any{"val": 4}

			inputPointPkg := &pkg.PointPackage{
				FrameId: baseFrameId,
				Ts:      baseTime,
				Points:  []*pkg.Point{pointA1, pointB1, pointA2, pointC1},
			}
			defer pkg.PointPoolInstance.Put(pointA1)
			defer pkg.PointPoolInstance.Put(pointB1)
			defer pkg.PointPoolInstance.Put(pointA2)
			defer pkg.PointPoolInstance.Put(pointC1)

			Convey("When Dispatch is called", func() {
				dispatchedPkgs, dispatchErr := handler.Dispatch(inputPointPkg)

				Reset(func() {
					for _, pkgItem := range dispatchedPkgs {
						for _, p := range pkgItem.Points {
							pkg.PointPoolInstance.Put(p)
						}
					}
				})

				Convey("Then it should correctly dispatch points to respective strategies", func() {
					So(dispatchErr, ShouldBeNil)
					// So(handler.LatestFrameId, ShouldEqual, baseFrameId) // Removed due to defer handler.Clean()
					// So(handler.LatestTs.Equal(baseTime), ShouldBeTrue)    // Removed due to defer handler.Clean()

					So(len(dispatchedPkgs), ShouldEqual, 2)

					pkgA, okA := dispatchedPkgs[strategyTypeA.Type]
					So(okA, ShouldBeTrue)
					So(pkgA, ShouldNotBeNil)
					So(pkgA.FrameId, ShouldEqual, baseFrameId)
					So(pkgA.Ts.Equal(baseTime), ShouldBeTrue)
					So(len(pkgA.Points), ShouldEqual, 2)
					foundA1, foundA2 := false, false
					for _, p := range pkgA.Points {
						So(p.Tag["type"], ShouldEqual, "A")
						if p.Tag["id"] == "A1" {
							foundA1 = true
							So(p, ShouldNotPointTo, pointA1)
						}
						if p.Tag["id"] == "A2" {
							foundA2 = true
							So(p, ShouldNotPointTo, pointA2)
						}
					}
					So(foundA1, ShouldBeTrue)
					So(foundA2, ShouldBeTrue)
					if len(pkgA.Points) == 2 {
						So(pkgA.Points[0], ShouldNotPointTo, pkgA.Points[1])
					}

					pkgB, okB := dispatchedPkgs[strategyTypeB.Type]
					So(okB, ShouldBeTrue)
					So(pkgB, ShouldNotBeNil)
					So(pkgB.FrameId, ShouldEqual, baseFrameId)
					So(pkgB.Ts.Equal(baseTime), ShouldBeTrue)
					So(len(pkgB.Points), ShouldEqual, 1)
					So(pkgB.Points[0].Tag["type"], ShouldEqual, "B")
					So(pkgB.Points[0].Tag["id"], ShouldEqual, "B1")
					So(pkgB.Points[0], ShouldNotPointTo, pointB1)
				})
			})
		})
	})
}

func TestHandlerClean(t *testing.T) {
	Convey("Testing handler.Clean", t, func() {
		Convey("Given a handler with some state and points in its pointList", func() {
			configs := []pkg.StrategyConfig{strategyAll}
			handler, err := NewHandler(configs)
			So(err, ShouldBeNil)

			handler.LatestTs = time.Now()
			handler.LatestFrameId = "frame-to-clean"

			p1 := pkg.PointPoolInstance.Get()
			p2 := pkg.PointPoolInstance.Get()
			// Manually populate handler.pointList as Dispatch/AddPoint don't use it for processed points
			handler.pointList = []*pkg.PointPackage{
				{FrameId: "f1", Ts: time.Now(), Points: []*pkg.Point{p1}},
				{FrameId: "f2", Ts: time.Now(), Points: []*pkg.Point{p2}},
			}

			Convey("When Clean is called", func() {
				handler.Clean() // This should Put p1 and p2 back
				Convey("Then handler state should be reset and pointList cleared", func() {
					So(handler.LatestTs.IsZero(), ShouldBeTrue)
					So(handler.LatestFrameId, ShouldEqual, "")
					So(len(handler.pointList), ShouldEqual, 0)
				})
			})
		})
		Convey("Given a handler with empty state", func() {
			configs := []pkg.StrategyConfig{}
			handler, err := NewHandler(configs)
			So(err, ShouldBeNil)
			Convey("When Clean is called", func() {
				handler.Clean()
				Convey("Then handler state should remain empty/zeroed", func() {
					So(handler.LatestTs.IsZero(), ShouldBeTrue)
					So(handler.LatestFrameId, ShouldEqual, "")
					So(len(handler.pointList), ShouldEqual, 0)
				})
			})
		})
	})
}
