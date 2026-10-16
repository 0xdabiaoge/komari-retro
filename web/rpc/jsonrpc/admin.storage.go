package jsonrpc

import (
	"context"
	"os"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func init() {
	reg("getDatabaseStorage", adminDatabaseStorage, "Get SQLite allocation and reclamation status")
}
func adminDatabaseStorage(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var pages, free, pageSize, mode int64
	db := dbcore.GetReadDB().WithContext(ctx)
	for query, dest := range map[string]*int64{"PRAGMA page_count": &pages, "PRAGMA freelist_count": &free, "PRAGMA page_size": &pageSize, "PRAGMA auto_vacuum": &mode} {
		if err := db.Raw(query).Scan(dest).Error; err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Unable to inspect database allocation", nil)
		}
	}
	wal := int64(0)
	if st, err := os.Stat(flags.DatabaseFile + "-wal"); err == nil {
		wal = st.Size()
	}
	loadHours, _ := config.GetAs[int]("record_preserve_time", 720)
	pingHours, _ := config.GetAs[int]("ping_record_preserve_time", 24)
	return map[string]any{"allocated_bytes": pages * pageSize, "reusable_bytes": free * pageSize, "wal_bytes": wal, "incremental_reclamation": mode == 2, "load_retention_hours": loadHours, "ping_retention_hours": pingHours}, nil
}
