package corn

import (
	"testing"
	"time"
)

func TestTrafficReportCalendarSchedules(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	cases := []struct {
		name string
		spec string
		from time.Time
		want time.Time
	}{
		{
			name: "daily at midnight",
			spec: "0 0 0 * * *",
			from: time.Date(2026, time.October, 7, 23, 59, 59, 0, location),
			want: time.Date(2026, time.October, 8, 0, 0, 0, 0, location),
		},
		{
			name: "weekly monday after sunday",
			spec: "0 0 0 * * 1",
			from: time.Date(2026, time.October, 11, 23, 59, 59, 0, location),
			want: time.Date(2026, time.October, 12, 0, 0, 0, 0, location),
		},
		{
			name: "weekly monday advances a week after midnight",
			spec: "0 0 0 * * 1",
			from: time.Date(2026, time.October, 12, 0, 0, 0, 0, location),
			want: time.Date(2026, time.October, 19, 0, 0, 0, 0, location),
		},
		{
			name: "monthly crosses year boundary",
			spec: "0 0 0 1 * *",
			from: time.Date(2026, time.December, 31, 23, 59, 59, 0, location),
			want: time.Date(2027, time.January, 1, 0, 0, 0, 0, location),
		},
		{
			name: "monthly handles leap day",
			spec: "0 0 0 1 * *",
			from: time.Date(2024, time.February, 29, 12, 0, 0, 0, location),
			want: time.Date(2024, time.March, 1, 0, 0, 0, 0, location),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schedule, err := Parse(tc.spec)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.spec, err)
			}
			if got := schedule.Next(tc.from); !got.Equal(tc.want) {
				t.Fatalf("Next(%s) = %s, want %s", tc.from, got, tc.want)
			}
		})
	}
}
