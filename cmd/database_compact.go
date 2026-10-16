package cmd

import (
	"fmt"
	"os"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(&cobra.Command{Use: "compact-database", Short: "离线整理 SQLite 并开启增量回收（先备份、停止服务；运行中的数据库会拒绝整理）", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		before, err := os.Stat(flags.DatabaseFile)
		if err != nil {
			return err
		}
		if err = dbcore.CompactOffline(flags.DatabaseFile); err != nil {
			return err
		}
		after, err := os.Stat(flags.DatabaseFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "数据库整理完成，完整性检查通过：%d -> %d 字节；已开启增量回收。\n", before.Size(), after.Size())
		return nil
	}})
}
