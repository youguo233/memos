package filter

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func tzCtx(t *testing.T, name string) context.Context {
	t.Helper()
	loc, ok := ParseLocation(name)
	require.True(t, ok, name)
	return WithLocation(context.Background(), loc)
}

func TestParseLocation(t *testing.T) {
	for _, bad := range []string{"", "Local", "Not/AZone", "../etc/passwd"} {
		_, ok := ParseLocation(bad)
		require.False(t, ok, bad)
	}
	loc, ok := ParseLocation("Asia/Shanghai")
	require.True(t, ok)
	require.Equal(t, "Asia/Shanghai", loc.String())
	require.Equal(t, time.UTC, LocationFromContext(context.Background()))
}

// mbe-tz-ui-v2: created_ts date accessors run in the viewer's zone on SQLite.
func TestTimestampAccessorTimezoneSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`CREATE TABLE memo (id INTEGER PRIMARY KEY, created_ts INTEGER, updated_ts INTEGER, payload TEXT)`)
	require.NoError(t, err)
	fixtures := map[int]string{
		1: "2025-12-31T20:00:00Z", // 2026-01-01 04:00 Shanghai; 15:00 New York (EST)
		2: "2026-07-01T03:00:00Z", // 2026-06-30 23:00 New York (EDT, -4)
		3: "2026-01-01T04:30:00Z", // 2025-12-31 23:30 New York (EST, -5)
	}
	for id, ts := range fixtures {
		v, err := time.Parse(time.RFC3339, ts)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO memo (id, created_ts, updated_ts, payload) VALUES (?, ?, ?, '{}')`, id, v.Unix(), v.Unix())
		require.NoError(t, err)
	}
	engine, err := NewEngine(NewSchema())
	require.NoError(t, err)

	ids := func(ctx context.Context, expr string) []int {
		stmt, err := engine.CompileToStatement(ctx, expr, RenderOptions{Dialect: DialectSQLite})
		require.NoError(t, err, expr)
		rows, err := db.Query("SELECT id FROM memo WHERE "+stmt.SQL+" ORDER BY id", stmt.Args...)
		require.NoError(t, err, stmt.SQL)
		defer rows.Close()
		var out []int
		for rows.Next() {
			var id int
			require.NoError(t, rows.Scan(&id))
			out = append(out, id)
		}
		return out
	}

	utc := context.Background()
	require.Equal(t, []int{2, 3}, ids(utc, `created_ts.getFullYear() == 2026`))
	require.Equal(t, []int{1, 2, 3}, ids(tzCtx(t, "Asia/Shanghai"), `created_ts.getFullYear() == 2026`))
	ny := tzCtx(t, "America/New_York")
	require.Equal(t, []int{2}, ids(ny, `created_ts.getFullYear() == 2026`))
	require.Equal(t, []int{2}, ids(ny, `created_ts.getMonth() == 5 && created_ts.getDate() == 30`))
	require.Equal(t, []int{2, 3}, ids(ny, `created_ts.getHours() == 23`))
}

func TestTimestampAccessorTimezoneDialects(t *testing.T) {
	engine, err := NewEngine(NewSchema())
	require.NoError(t, err)
	ctx := tzCtx(t, "Asia/Tokyo")
	pg, err := engine.CompileToStatement(ctx, `created_ts.getFullYear() == 2026`, RenderOptions{Dialect: DialectPostgres})
	require.NoError(t, err)
	require.Contains(t, pg.SQL, "AT TIME ZONE 'Asia/Tokyo'")
	my, err := engine.CompileToStatement(ctx, `created_ts.getFullYear() == 2026`, RenderOptions{Dialect: DialectMySQL})
	require.NoError(t, err)
	require.Contains(t, my.SQL, "CONVERT_TZ(")
	require.Contains(t, my.SQL, "INTERVAL")
	// UTC output is unchanged.
	plain, err := engine.CompileToStatement(context.Background(), `created_ts.getFullYear() == 2026`, RenderOptions{Dialect: DialectMySQL})
	require.NoError(t, err)
	require.False(t, strings.Contains(plain.SQL, "CONVERT_TZ"))
}

func TestNowAccessorTimezone(t *testing.T) {
	engine, err := NewEngine(NewSchema())
	require.NoError(t, err)
	engine.nowFunc = func() time.Time { return time.Date(2025, 12, 31, 20, 0, 0, 0, time.UTC) }
	year := func(ctx context.Context) any {
		p, err := engine.Compile(ctx, `created_ts.getFullYear() == now.getFullYear()`)
		require.NoError(t, err)
		st, err := p.Render(RenderOptions{Dialect: DialectSQLite})
		require.NoError(t, err)
		return st.Args[len(st.Args)-1]
	}
	require.EqualValues(t, 2025, year(context.Background()))
	require.EqualValues(t, 2026, year(tzCtx(t, "Asia/Shanghai")))
}
