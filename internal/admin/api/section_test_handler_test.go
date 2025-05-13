package api_test

import (
	"bytes"
	"encoding/json"
	"gateway/internal/admin/api"
	"gateway/internal/admin/router" // Import router to set up the engine
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/smartystreets/goconvey/convey"
)

// Helper function to perform test requests
func performTestSectionRequest(t *testing.T, r *gin.Engine, reqBody api.TestSectionRequest) (*httptest.ResponseRecorder, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "/api/v1/test/section", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Logf("Received unexpected status code: %d. Response Body: %s", w.Code, w.Body.String())
	}
	return w, nil
}

func TestTestSectionHandler(t *testing.T) {
	Convey("TestSectionHandler API Endpoint Tests", t, func() {
		// Set Gin to Test Mode
		gin.SetMode(gin.TestMode)

		// Set up Router
		r := router.SetupRouter() // Use the actual router setup

		Convey("Success Case - Basic Single Section", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"desc": "Basic Section",
					"size": 1,
					// Replace Dev with Points
					"Points": []map[string]interface{}{
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""}, // Use `id` tag
							"Field": map[string]interface{}{"value": "Bytes[0]"},
						},
					},
				},
				HexPayload: "01",
			}

			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			So(len(respBody.Points), ShouldBeGreaterThan, 0)
			// Check point.Tag["id"] instead of point.Device
			foundDevice := false
			for _, point := range respBody.Points {
				if id, ok := point.Tag["id"].(string); ok && id == "dev1" {
					foundDevice = true
					break
				}
			}
			So(foundDevice, ShouldBeTrue)

			// 验证dispatcher处理结果
			So(respBody.DispatcherResults, ShouldNotBeNil)
		})

		Convey("Success Case - Using SectionConfigs Array with Dispatcher", func() {
			reqBody := api.TestSectionRequest{
				SectionConfigs: []map[string]interface{}{
					{
						"desc": "第一个Section",
						"size": 1,
						"Points": []map[string]interface{}{ // Use Points
							{
								"Tag":   map[string]interface{}{"id": "\"dev1\""}, // Use id tag
								"Field": map[string]interface{}{"value": "Bytes[0]"},
							},
						},
					},
					{
						"desc": "第二个Section",
						"size": 1,
						"Points": []map[string]interface{}{ // Use Points
							{
								"Tag":   map[string]interface{}{"id": "\"dev2\""}, // Use id tag
								"Field": map[string]interface{}{"value": "Bytes[0]"},
							},
						},
					},
				},
				HexPayload: "0102",
			}

			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			So(len(respBody.Points), ShouldBeGreaterThan, 0)
			// Check point.Tag["id"]
			dev1Found, dev2Found := false, false
			for _, point := range respBody.Points {
				if id, ok := point.Tag["id"].(string); ok {
					if id == "dev1" {
						dev1Found = true
					} else if id == "dev2" {
						dev2Found = true
					}
				}
			}
			So(dev1Found, ShouldBeTrue)
			So(dev2Found, ShouldBeTrue)

			// Check dispatcher results using Tag["id"]
			So(respBody.DispatcherResults, ShouldNotBeNil)
			dispatcherDevs := make(map[string]bool)
			for _, strategyResult := range respBody.DispatcherResults {
				for _, point := range strategyResult.Points {
					if id, ok := point.Tag["id"].(string); ok {
						dispatcherDevs[id] = true
					}
				}
			}
			t.Logf("Dispatcher处理结果中包含的设备 (based on Tag['id']): %v", dispatcherDevs)
		})

		Convey("Failure Case - Invalid Hex Payload (Odd Length)", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"size": 1,
					"Points": []map[string]interface{}{ // Minimal valid Points
						{"Tag": map[string]interface{}{"id": "\"min\""}, "Field": map[string]interface{}{"f": "Bytes[0]"}},
					},
				},
				HexPayload: "123", // Odd length
			}
			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusBadRequest)

			var errResp map[string]string
			err = json.Unmarshal(w.Body.Bytes(), &errResp)
			So(err, ShouldBeNil)
			So(errResp["error"], ShouldContainSubstring, "length must be even")
		})

		Convey("Failure Case - Invalid Section Config (Missing Size)", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					// Missing "size" field
					"Points": []map[string]interface{}{
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""},
							"Field": map[string]interface{}{"val": "Bytes[0]"},
						},
					},
				},
				HexPayload: "01",
			}
			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			// Building the parser/sequence should fail early
			So(w.Code, ShouldEqual, http.StatusBadRequest) // Expect Bad Request because parser creation fails

			var errResp map[string]string
			err = json.Unmarshal(w.Body.Bytes(), &errResp)
			So(err, ShouldBeNil)
			// Check for parser creation failure message
			So(errResp["error"], ShouldContainSubstring, "Failed to create parser")
		})

		Convey("Failure Case - Processing Error (Data Too Short)", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"size": 2, // Requires 2 bytes
					"Points": []map[string]interface{}{ // Use Points
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""},
							"Field": map[string]interface{}{"val": "Bytes[0]"},
						},
					},
				},
				HexPayload: "01", // Only 1 byte provided
			}
			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			// Processing error now returns 200 OK but with error details inside the response body
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			// Check for the processing error within the steps
			foundError := false
			for _, step := range respBody.ProcessingSteps {
				if step.Error != "" {
					So(step.Error, ShouldContainSubstring, "数据不足") // Check for underlying error message
					foundError = true
					break
				}
			}
			So(foundError, ShouldBeTrue)
		})

		Convey("Success Case - With InitialVars", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"desc": "UseInitialVar",
					"size": 1,
					"Points": []map[string]interface{}{ // Use Points
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""},
							"Field": map[string]interface{}{"output": "Bytes[0]"},
						},
					},
				},
				HexPayload:  "05",
				InitialVars: map[string]interface{}{"start_value": 10},
			}

			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			So(len(respBody.Points), ShouldBeGreaterThan, 0)
			So(respBody.FinalVars, ShouldContainKey, "start_value")
			startVal, ok := respBody.FinalVars["start_value"].(float64) // JSON numbers are float64
			So(ok, ShouldBeTrue)
			So(startVal, ShouldEqual, 10)
		})

		Convey("Success Case - With GlobalMap", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"desc": "UseGlobalMap",
					"size": 1,
					"Points": []map[string]interface{}{ // Use Points
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""},
							"Field": map[string]interface{}{"output": "Bytes[0]"},
						},
					},
				},
				HexPayload: "0A",
				GlobalMap:  map[string]interface{}{"multiplier": 2.5},
			}

			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			So(len(respBody.Points), ShouldBeGreaterThan, 0)
			// GlobalMap values are not directly in FinalVars unless explicitly set by an expression
			So(respBody.FinalVars, ShouldNotContainKey, "multiplier")
		})

		Convey("Success Case - With Both InitialVars and GlobalMap", func() {
			reqBody := api.TestSectionRequest{
				SectionConfig: map[string]interface{}{
					"desc": "UseBothContexts",
					"size": 1,
					"Points": []map[string]interface{}{ // Use Points
						{
							"Tag":   map[string]interface{}{"id": "\"dev1\""},
							"Field": map[string]interface{}{"output": "Bytes[0]"},
						},
					},
				},
				HexPayload:  "02",
				InitialVars: map[string]interface{}{"offset": 5},
				GlobalMap:   map[string]interface{}{"scale": 100.0},
			}

			w, err := performTestSectionRequest(t, r, reqBody)
			So(err, ShouldBeNil)
			So(w.Code, ShouldEqual, http.StatusOK)

			var respBody api.TestSectionResponse
			err = json.Unmarshal(w.Body.Bytes(), &respBody)
			So(err, ShouldBeNil)

			So(len(respBody.Points), ShouldBeGreaterThan, 0)
			So(respBody.FinalVars, ShouldContainKey, "offset")
			offsetVal, ok := respBody.FinalVars["offset"].(float64) // JSON numbers are float64
			So(ok, ShouldBeTrue)
			So(offsetVal, ShouldEqual, 5)
		})
	})
}
