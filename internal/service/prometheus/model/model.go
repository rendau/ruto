package model

type Series struct {
	Labels map[string]string
	Points []Point
}

type Point struct {
	Ts    int64 // unix seconds
	Value float64
}
