# Database Seeder

This seeder populates the database with sample data for development and testing purposes.

## What it seeds

1. **Users** (10 users):
   - 1 Admin user (`admin@hamsaya.af` / `Admin123!`)
   - 9 Regular users (John Doe, Jane Smith, Ahmad Khan, etc.)
   - All users have verified emails and profiles with first/last names

2. **Categories** (10 categories):
   - Electronics, Fashion, Home & Garden, Sports, Books
   - Automotive, Food & Beverages, Health & Beauty, Toys & Games, Real Estate
   - Each with appropriate icons and colors

3. **Businesses** (8 businesses):
   - Kabul Coffee House, Afghan Handicrafts, Mazar Restaurant
   - Tech Solutions AF, Kandahar Textiles, Herat Carpets
   - Jalalabad Foods, Ghazni Pottery
   - Each with license numbers, contact info, and locations

4. **Posts** (11 posts):
   - 3 FEED posts (community updates, recipes, etc.)
   - 3 EVENT posts (community gathering, tech meetup, coffee tasting)
   - 3 SELL posts (laptop, traditional dress, cotton fabric)
   - 2 PULL/Poll posts (favorite dish poll, weekend plans)

## Usage

```bash
# Run the seeder
make seed

# Or run directly
go run cmd/seed/main.go
```

## Important Notes

- **Database must be running**: Ensure PostgreSQL is running via `docker-compose up -d postgres`
- **Migrations must be applied**: Run `make migrate-up` before seeding
- **Idempotency**: The seeder will fail if data already exists (email uniqueness)
- **Clean database**: To re-seed, truncate tables or reset the database first

## Resetting the database

If you need to clear existing data and re-seed:

```bash
# Option 1: Drop and recreate database (requires migrations to be reapplied)
docker-compose down -v
docker-compose up -d postgres redis minio
make migrate-up
make seed

# Option 2: Manually truncate tables (be careful!)
docker exec -it hamsaya-postgres psql -U hamsaya -d hamsaya_db
# Then run TRUNCATE commands for each table
```

## Credentials

Admin user:
- Email: `admin@hamsaya.af`
- Password: `Admin123!`

Regular users:
- Email: `john.doe@example.com`, `jane.smith@example.com`, etc.
- Password: `Password123!` (same for all test users)

## Extending the seeder

To add more seed data:

1. Add data to the appropriate seeder function in `main.go`
2. Follow the existing patterns for generating UUIDs and timestamps
3. Ensure proper error handling
4. Update this README with the new data
