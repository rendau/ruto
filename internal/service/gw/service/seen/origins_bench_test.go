package seen

import "testing"

// the request-path cost of Record: a non-blocking channel send
func BenchmarkOrigins_Record(b *testing.B) {
	s := NewOrigins()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Record("https://App.Example.com")
		}
	})
	b.ReportMetric(float64(s.Dropped())/float64(b.N)*100, "%dropped")
}

// the consumer side: how fast queued headers are parsed, folded and merged
func BenchmarkOrigins_Consume(b *testing.B) {
	s := &Origins{queue: make(chan string, originQueueSize), items: make(map[string]*OriginItem)}
	raws := []string{"https://App.Example.com", "http://localhost:5173", "https://b.example.com", "null"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i += len(raws) {
		for _, raw := range raws {
			s.queue <- raw
		}
		if len(s.queue) >= originQueueSize-len(raws) || i+len(raws) >= b.N {
			close(s.queue)
			s.run()
			s.queue = make(chan string, originQueueSize)
		}
	}
}
