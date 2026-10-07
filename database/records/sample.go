package records

import (
	"context"
	"fmt"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"math"
	"time"
)

// Sampling happens in SQLite, so a long window never materializes every row in Go.
func SampleLoad(ctx context.Context, uuid string, start, end time.Time, limit int) ([]models.Record, error) {
	if limit <= 0 || limit > 10000 || !start.Before(end) || end.Sub(start) > 31*24*time.Hour {
		return nil, fmt.Errorf("invalid history query bounds")
	}
	db := dbcore.GetDBInstance().WithContext(ctx)
	var nodes int64 = 1
	if uuid == "" {
		if err := db.Model(&models.Client{}).Count(&nodes).Error; err != nil {
			return nil, err
		}
		if nodes < 1 {
			nodes = 1
		}
	}
	bucket := int64(math.Ceil(end.Sub(start).Seconds() * float64(nodes) / float64(limit)))
	if bucket < 1 {
		bucket = 1
	}
	boundary := time.Now().Add(-4*time.Hour - time.Minute)
	sql := `WITH source AS (
 SELECT * FROM records WHERE time>=? AND time<=? AND time>=? AND (?='' OR client=?)
 UNION ALL SELECT * FROM records_long_term WHERE time>=? AND time<=? AND time<? AND (?='' OR client=?)
 ), sampled AS (
 SELECT *, ROW_NUMBER() OVER (PARTITION BY client, CAST(strftime('%s',time) AS INTEGER)/? ORDER BY time DESC) AS sample_rank FROM source
 ) SELECT * FROM sampled WHERE sample_rank=1 ORDER BY time ASC LIMIT ?`
	var result []models.Record
	err := db.Raw(sql, start, end, boundary, uuid, uuid, start, end, boundary, uuid, uuid, bucket, limit).Scan(&result).Error
	return result, err
}
