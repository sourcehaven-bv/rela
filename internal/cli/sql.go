package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/sqldb"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

const defaultMySQLPort = 3306

var (
	sqlHost     string
	sqlPort     int
	sqlSocket   string
	sqlUser     string
	sqlPassword string
	sqlDatabase string
)

// coverage-ignore: SQL server command - requires network server
var sqlCmd = &cobra.Command{
	Use:   "sql",
	Short: "Start a MySQL-compatible SQL server",
	Long: `Starts a MySQL-compatible SQL server exposing the rela graph.

Each entity type becomes a table (pluralized name) with columns for id,
properties, and content. Each relation type also becomes a table with
from_id, to_id, properties, and content columns.

Examples:
  rela sql                          # Start on default port 3306
  rela sql --port 3307              # Start on custom port
  rela sql --host 0.0.0.0           # Listen on all interfaces
  rela sql --socket /tmp/rela.sock  # Listen on Unix socket

Connect with any MySQL client:
  mysql -h localhost -P 3306 -u root rela
  mysql --socket /tmp/rela.sock -u root rela

Query examples:
  SELECT * FROM requirements;
  SELECT * FROM implements WHERE from_id = 'COMP-001';
  SELECT r.id, r.title, r.status FROM requirements r
    JOIN implements i ON r.id = i.to_id
    WHERE i.from_id = 'COMP-001';`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSQLServer()
	},
}

func runSQLServer() error {
	// Determine start directory: flag > env var > cwd
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	// Discover project and initialize workspace
	sqlWs, err := workspace.DiscoverAndNew(startDir)
	if err != nil {
		return fmt.Errorf("no project found: run 'rela init' to create one")
	}

	config := sqldb.ServerConfig{
		Host:     sqlHost,
		Port:     sqlPort,
		Socket:   sqlSocket,
		User:     sqlUser,
		Password: sqlPassword,
		Database: sqlDatabase,
	}

	srv, err := sqldb.NewServer(sqlWs.Graph(), sqlWs.Meta(), config)
	if err != nil {
		return fmt.Errorf("failed to create SQL server: %w", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "Starting MySQL server on %s\n", srv.Address())
	if sqlSocket != "" {
		fmt.Fprintf(os.Stderr, "Connect with: mysql --socket %s -u %s %s\n", sqlSocket, sqlUser, sqlDatabase)
	} else {
		fmt.Fprintf(os.Stderr, "Connect with: mysql -h %s -P %d -u %s %s\n", sqlHost, sqlPort, sqlUser, sqlDatabase)
	}

	return srv.Start()
}

func init() {
	sqlCmd.Flags().StringVar(&sqlHost, "host", "localhost", "Host to listen on")
	sqlCmd.Flags().IntVar(&sqlPort, "port", defaultMySQLPort, "Port to listen on")
	sqlCmd.Flags().StringVar(&sqlSocket, "socket", "", "Unix socket path (overrides host/port)")
	sqlCmd.Flags().StringVar(&sqlUser, "user", "root", "MySQL user")
	sqlCmd.Flags().StringVar(&sqlPassword, "password", "", "MySQL password (empty for no auth)")
	sqlCmd.Flags().StringVar(&sqlDatabase, "database", "rela", "Database name")

	rootCmd.AddCommand(sqlCmd)
}
