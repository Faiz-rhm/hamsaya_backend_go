# Run Database Migration

## Quick Start

To apply the user roles migration, run:

```bash
# Option 1: Using make (recommended)
make migrate-up

# Option 2: Using Docker directly
docker-compose up -d postgres
sleep 10  # Wait for database to be ready
docker-compose exec postgres psql -U postgres -d hamsaya -f /docker-entrypoint-initdb.d/20240102000001_add_user_roles.up.sql

# Option 3: Using the migrate CLI
bin/migrate up
```

## What This Migration Does

Adds the `role` column to the `users` table:
- Creates `user_role` enum type with values: `user`, `admin`, `moderator`
- Adds `role` column with default value `user`
- Creates index on role column for fast queries

## Verify Migration

```bash
# Check if migration was applied
docker-compose exec postgres psql -U postgres -d hamsaya -c "\d users"

# You should see the 'role' column in the output
```

## Create Your First Admin User

```bash
# After migration, promote a user to admin:
docker-compose exec postgres psql -U postgres -d hamsaya -c "UPDATE users SET role = 'admin' WHERE email = 'your@email.com';"
```

## Rollback (if needed)

```bash
make migrate-down
```
