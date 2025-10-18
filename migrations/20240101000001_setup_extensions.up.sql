-- Enable required PostgreSQL extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "postgis";

-- Create schema_migrations table will be created automatically by the migrator
-- This migration sets up the basic extensions needed for the application
