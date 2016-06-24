This is a reporter for the [go-metrics](https://github.com/launchdarkly/go-metrics)
library which will post the metrics to Graphite. It was originally part of the
`go-metrics` library itself, but has been split off to make maintenance of
both the core library and the client easier.

### Usage

```go
import "github.com/launchdarkly/go-metrics-graphite"


go graphite.Graphite(metrics.DefaultRegistry,
  1*time.Second, "some.prefix", addr)
```

