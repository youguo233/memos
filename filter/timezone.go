package filter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// TimezoneHeader carries the viewer's IANA time zone (their preferred zone, or the
// browser's on "System"). CEL date accessors (created_ts.getFullYear(), getMonth(),
// now.getDate(), ...) are evaluated in that zone. Storage stays UTC. Marker: mbe-tz-ui-v2.
const TimezoneHeader = "X-Memos-Timezone"

type locationKey struct{}

// WithLocation returns ctx carrying the zone CEL date accessors are evaluated in.
func WithLocation(ctx context.Context, loc *time.Location) context.Context {
	if loc == nil {
		return ctx
	}
	return context.WithValue(ctx, locationKey{}, loc)
}

// LocationFromContext is the zone set by WithLocation, or UTC.
func LocationFromContext(ctx context.Context) *time.Location {
	if ctx != nil {
		if loc, ok := ctx.Value(locationKey{}).(*time.Location); ok && loc != nil {
			return loc
		}
	}
	return time.UTC
}

// ParseLocation loads an IANA zone name; empty, "Local" or unknown names give false.
func ParseLocation(name string) (*time.Location, bool) {
	name = strings.TrimSpace(name)
	if name == "" || name == "Local" || len(name) > 64 {
		return nil, false
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, false
	}
	return loc, true
}

func isUTC(loc *time.Location) bool {
	if loc == nil || loc == time.UTC {
		return true
	}
	name := loc.String()
	return name == "UTC" || name == "Etc/UTC"
}

// zoneSegment is a period with a fixed UTC offset, starting at `from` (epoch seconds).
type zoneSegment struct {
	from   int64
	offset int
}

var segmentCache sync.Map // zone name -> []zoneSegment

// zoneSegments lists the offset periods of loc from 1900 to 20 years from now.
func zoneSegments(loc *time.Location) []zoneSegment {
	if cached, ok := segmentCache.Load(loc.String()); ok {
		return cached.([]zoneSegment)
	}
	t := time.Date(1900, 1, 1, 0, 0, 0, 0, loc)
	limit := time.Now().AddDate(20, 0, 0)
	_, off := t.Zone()
	segs := []zoneSegment{{from: -1 << 62, offset: off}}
	for i := 0; i < 2000; i++ {
		_, end := t.ZoneBounds()
		if end.IsZero() || end.After(limit) {
			break
		}
		t = end
		_, off = t.Zone()
		if off != segs[len(segs)-1].offset {
			segs = append(segs, zoneSegment{from: t.Unix(), offset: off})
		}
	}
	segmentCache.Store(loc.String(), segs)
	return segs
}

// offsetSQL is an SQL integer expression: loc's UTC offset in seconds at `epoch`
// (an SQL expression in epoch seconds), honoring daylight saving transitions.
func offsetSQL(loc *time.Location, epoch string) string {
	segs := zoneSegments(loc)
	if len(segs) == 1 {
		return fmt.Sprintf("%d", segs[0].offset)
	}
	var b strings.Builder
	b.WriteString("(CASE")
	for i := len(segs) - 1; i >= 1; i-- {
		fmt.Fprintf(&b, " WHEN %s >= %d THEN %d", epoch, segs[i].from, segs[i].offset)
	}
	fmt.Fprintf(&b, " ELSE %d END)", segs[0].offset)
	return b.String()
}

// LocationOrUTC returns loc, or UTC when loc is nil.
func LocationOrUTC(loc *time.Location) *time.Location {
	if loc == nil {
		return time.UTC
	}
	return loc
}
