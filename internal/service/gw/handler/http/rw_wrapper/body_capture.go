package rw_wrapper

import "io"

const truncatedSuffix = "...(truncated)"

// BodyCapture accumulates up to limit bytes of a stream for logging, flagging
// when the stream exceeded the limit so the log can show it was truncated.
type BodyCapture struct {
	buf       []byte
	limit     int
	truncated bool
}

func NewBodyCapture(limit int) *BodyCapture {
	return &BodyCapture{limit: limit}
}

func (c *BodyCapture) Write(p []byte) {
	if c == nil || len(p) == 0 {
		return
	}
	remaining := c.limit - len(c.buf)
	if remaining <= 0 {
		c.truncated = true
		return
	}
	if len(p) > remaining {
		c.buf = append(c.buf, p[:remaining]...)
		c.truncated = true
		return
	}
	c.buf = append(c.buf, p...)
}

func (c *BodyCapture) String() string {
	if c == nil {
		return ""
	}
	if c.truncated {
		return string(c.buf) + truncatedSuffix
	}
	return string(c.buf)
}

// WrapReader returns a ReadCloser that copies what is read into the capture
// (up to the limit) while passing the full stream through to the consumer.
func (c *BodyCapture) WrapReader(rc io.ReadCloser) io.ReadCloser {
	return &captureReadCloser{rc: rc, cap: c}
}

type captureReadCloser struct {
	rc  io.ReadCloser
	cap *BodyCapture
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if n > 0 {
		r.cap.Write(p[:n])
	}
	return n, err
}

func (r *captureReadCloser) Close() error {
	return r.rc.Close()
}
