// Package sqldb provides a MySQL-compatible SQL interface to rela graphs.
package sqldb

import (
	"fmt"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/go-mysql-server/sql"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Server wraps a go-mysql-server instance serving a rela graph.
type Server struct {
	engine *sqle.Engine
	server *server.Server
	config ServerConfig
}

// ServerConfig holds configuration for the SQL server.
type ServerConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// DefaultConfig returns default server configuration.
func DefaultConfig() ServerConfig {
	return ServerConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "",
		Database: "rela",
	}
}

// NewServer creates a new SQL server for the given graph and metamodel.
func NewServer(g *graph.Graph, meta *metamodel.Metamodel, config ServerConfig) (*Server, error) {
	// Create the rela database
	db := NewDatabase(config.Database, g, meta)

	// Create a database provider
	provider := memory.NewDBProvider(db)

	// Create the SQL engine
	engine := sqle.NewDefault(provider)

	// Configure authentication (allow root without password for simplicity)
	engine.Analyzer.Catalog.MySQLDb.SetEnabled(true)
	engine.Analyzer.Catalog.MySQLDb.AddRootAccount()

	// Create server config
	serverConfig := server.Config{
		Protocol: "tcp",
		Address:  fmt.Sprintf("%s:%d", config.Host, config.Port),
	}

	// Create the MySQL server
	srv, err := server.NewServer(
		serverConfig,
		engine,
		sql.NewContext,
		memory.NewSessionBuilder(provider),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return &Server{
		engine: engine,
		server: srv,
		config: config,
	}, nil
}

// Start starts the SQL server.
func (s *Server) Start() error {
	return s.server.Start()
}

// Close stops the SQL server.
func (s *Server) Close() error {
	return s.server.Close()
}

// Address returns the server address.
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}
