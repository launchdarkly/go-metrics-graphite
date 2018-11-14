package graphite

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/launchdarkly/go-metrics"
)

// GraphiteConfig provides a container with configuration parameters for
// the Graphite exporter
type GraphiteConfig struct {
	Addr          string           // Network address to connect to in the form of host:port
	Registry      metrics.Registry // Registry to be exported
	FlushInterval time.Duration    // Flush interval
	DurationUnit  time.Duration    // Time conversion unit for durations
	Prefix        string           // Prefix to be prepended to metric names
	Percentiles   []float64        // Percentiles to export from timers and histograms
}

// Graphite is a blocking exporter function which reports metrics in r
// to a graphite server located at addr (in the form of host:port), flushing them every d duration
// and prepending metric names with prefix.
func Graphite(r metrics.Registry, d time.Duration, prefix string, addr string) {
	GraphiteWithConfig(GraphiteConfig{
		Addr:          addr,
		Registry:      r,
		FlushInterval: d,
		DurationUnit:  time.Nanosecond,
		Prefix:        prefix,
		Percentiles:   []float64{0.5, 0.75, 0.95, 0.99, 0.999},
	})
}

// GraphiteWithConfig is a blocking exporter function just like Graphite,
// but it takes a GraphiteConfig instead.
func GraphiteWithConfig(c GraphiteConfig) {
	for range time.Tick(c.FlushInterval) {
		if err := graphite(&c); nil != err {
			log.Println(err)
		}
	}
}

// GraphiteOnce performs a single submission to Graphite, returning a
// non-nil error on failed connections. This can be used in a loop
// similar to GraphiteWithConfig for custom error handling.
func GraphiteOnce(c GraphiteConfig) error {
	return graphite(&c)
}

func graphite(c *GraphiteConfig) error {
	now := time.Now().Unix()
	du := float64(c.DurationUnit)
	tcpAddr, err := net.ResolveTCPAddr("tcp", c.Addr)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if nil != err {
		return err
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			ctr := metric.Clear()
			_, err = fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, ctr.Count(), now)
		case metrics.GaugeCounter:
			_, err = fmt.Fprintf(w, "%s.%s.value %d %d\n", c.Prefix, name, metric.Count(), now)
		case metrics.Gauge:
			_, err = fmt.Fprintf(w, "%s.%s.value %d %d\n", c.Prefix, name, metric.Value(), now)
		case metrics.GaugeFloat64:
			_, err = fmt.Fprintf(w, "%s.%s.value %f %d\n", c.Prefix, name, metric.Value(), now)
		case metrics.Histogram:
			h := metric.Clear() // atomically clears and returns snapshot of values before clearing.
			ps := h.Percentiles(c.Percentiles)
			_, err = fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, h.Count(), now)
			_, err = fmt.Fprintf(w, "%s.%s.min %d %d\n", c.Prefix, name, h.Min(), now)
			_, err = fmt.Fprintf(w, "%s.%s.max %d %d\n", c.Prefix, name, h.Max(), now)
			_, err = fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, h.Mean(), now)
			_, err = fmt.Fprintf(w, "%s.%s.std-dev %.2f %d\n", c.Prefix, name, h.StdDev(), now)
			for psIdx, psKey := range c.Percentiles {
				key := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				_, err = fmt.Fprintf(w, "%s.%s.%s-percentile %.2f %d\n", c.Prefix, name, key, ps[psIdx], now)
			}
		case metrics.Meter:
			m := metric.Snapshot()
			_, err = fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, m.Count(), now)
			_, err = fmt.Fprintf(w, "%s.%s.one-minute %.2f %d\n", c.Prefix, name, m.Rate1(), now)
			_, err = fmt.Fprintf(w, "%s.%s.five-minute %.2f %d\n", c.Prefix, name, m.Rate5(), now)
			_, err = fmt.Fprintf(w, "%s.%s.fifteen-minute %.2f %d\n", c.Prefix, name, m.Rate15(), now)
			_, err = fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, m.RateMean(), now)
		case metrics.Timer:
			t := metric.Clear() // atomically clears and returns snapshot of values before clearing.
			ps := t.Percentiles(c.Percentiles)
			_, err = fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, t.Count(), now)
			_, err = fmt.Fprintf(w, "%s.%s.min %d %d\n", c.Prefix, name, t.Min()/int64(du), now)
			_, err = fmt.Fprintf(w, "%s.%s.max %d %d\n", c.Prefix, name, t.Max()/int64(du), now)
			_, err = fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, t.Mean()/du, now)
			_, err = fmt.Fprintf(w, "%s.%s.std-dev %.2f %d\n", c.Prefix, name, t.StdDev()/du, now)
			for psIdx, psKey := range c.Percentiles {
				key := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				_, err = fmt.Fprintf(w, "%s.%s.%s-percentile %.2f %d\n", c.Prefix, name, key, ps[psIdx]/du, now)
			}
		default:
			err = fmt.Errorf("Error sending metrics to Graphite: Got unknown metric type: %+v", i)
		}
		err = w.Flush()
	})
	return err
}
