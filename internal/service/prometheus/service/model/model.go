package model

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/samber/lo"

	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

type QueryRangeRep struct {
	Status    string         `json:"status"`
	ErrorType string         `json:"errorType"`
	Error     string         `json:"error"`
	Data      QueryRangeData `json:"data"`
}

type QueryRangeData struct {
	ResultType string             `json:"resultType"`
	Result     []QueryRangeSeries `json:"result"`
}

type QueryRangeSeries struct {
	Metric map[string]string    `json:"metric"`
	Values [][2]json.RawMessage `json:"values"`
}

func DecodeSeries(v QueryRangeSeries, _ int) prometheusModel.Series {
	return prometheusModel.Series{
		Labels: v.Metric,
		Points: lo.FilterMap(v.Values, decodePoint),
	}
}

func decodePoint(v [2]json.RawMessage, _ int) (prometheusModel.Point, bool) {
	var ts float64
	if err := json.Unmarshal(v[0], &ts); err != nil {
		return prometheusModel.Point{}, false
	}

	var valueStr string
	if err := json.Unmarshal(v[1], &valueStr); err != nil {
		return prometheusModel.Point{}, false
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return prometheusModel.Point{}, false
	}

	// prometheus yields NaN for 0/0 divisions (e.g. avg duration with no
	// traffic) — drop such points instead of leaking non-finite values to JSON
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return prometheusModel.Point{}, false
	}

	return prometheusModel.Point{
		Ts:    int64(ts),
		Value: value,
	}, true
}
