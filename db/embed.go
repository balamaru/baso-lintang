package db

import "embed"

// Migrations contains the versioned database schema.
//
//go:embed migrations/*.sql
var Migrations embed.FS
