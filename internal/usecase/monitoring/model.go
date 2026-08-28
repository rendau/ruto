package monitoring

type Status struct {
	MetricsEnabled bool
	LogsEnabled    bool
	LogsProvider   string
}

type Series struct {
	StepSeconds int64
	Points      []SeriesPoint
}

type SeriesPoint struct {
	Ts                 int64
	Rps                float64
	DurationAvgSeconds float64
	ErrorRate          float64
}

type EndpointsRps struct {
	StepSeconds int64
	Results     []EndpointRps
}

type EndpointRps struct {
	EndpointId string
	Points     []ValuePoint
}

type ValuePoint struct {
	Ts    int64
	Value float64
}

type LogEntry struct {
	TsMs     int64
	Status   string
	Duration string
	Error    string
	Message  string
	Raw      string
}
