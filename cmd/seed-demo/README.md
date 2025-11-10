# Demo Database Seeder

This comprehensive demo seeder populates the database with realistic, interconnected data perfect for demonstrations, testing, and development.

## What it seeds

### 1. **Users** (15 users)
- 1 Demo Admin user
- 14 Regular demo users (11 active, 4 inactive)
- All with verified emails and complete profiles
- Inactive users marked with `deleted_at` timestamp for testing user management

### 2. **Categories** (12 categories)
- Electronics, Fashion & Clothing, Home & Garden
- Sports & Outdoors, Books & Education, Automotive
- Food & Beverages, Health & Beauty, Toys & Kids
- Real Estate, Furniture, Services
- Each with appropriate Material Icons and colors

### 3. **Businesses** (10 businesses)
- Kabul Tech Hub, Afghan Heritage Gallery
- Balkh Cuisine Restaurant, Digital Solutions Afghanistan
- Kandahar Fashion House, Herat Artisan Crafts
- Jalalabad Fresh Foods, Ghazni Traditional Arts
- Kabul Coffee Lounge, Afghan Bookstore
- Each with complete contact info and locations

### 4. **Posts** (20 posts)
- **5 FEED posts**: Community updates, recipes, tech discussions
- **5 EVENT posts**: Meetups, community gatherings, workshops
- **5 SELL posts**: Electronics, clothing, books with realistic descriptions
- **5 PULL posts**: Polls about food, activities, preferences

### 5. **Poll Options & Votes**
- Each poll has 4 options
- Realistic vote distribution across users
- Examples: "Best Afghan dish", "Weekend preferences", "Coffee styles"

### 6. **Comments** (10+ comments)
- Realistic comments on various posts
- Natural conversation threads
- Community engagement examples

### 7. **Likes**
- First 10 posts have 3-7 likes each
- Distributed across different users
- Realistic engagement patterns

### 8. **Event Interests**
- Users marked as "interested" (5 per event)
- Users marked as "going" (3 per event)
- Demonstrates event management features

### 9. **User Relationships**
- 20+ follow relationships
- Creates a realistic social network
- Users following each other bidirectionally

### 10. **Reports** (14 reports)
- **5 Post Reports**: Spam, inappropriate content, misleading info, harassment
- **3 Comment Reports**: Harassment, spam, inappropriate content
- **3 User Reports**: Harassment, fake accounts, spam
- **3 Business Reports**: Fake business, misleading info, inappropriate content
- Mixed statuses: PENDING, REVIEWING, RESOLVED, REJECTED
- Demonstrates content moderation system

## Key Features

✅ **Idempotent**: Can be run multiple times safely
- Checks if users exist before creating
- Handles duplicate data gracefully
- Uses `ON CONFLICT DO NOTHING` where appropriate

✅ **Realistic Data**:
- Afghan-themed content and businesses
- Proper timestamps (spread over time)
- Natural engagement patterns
- Interconnected relationships

✅ **Complete Coverage**:
- Tests all 4 post types (FEED, EVENT, SELL, PULL)
- Multiple user interactions
- Business and user posts
- Comments, likes, and follows

## Usage

```bash
# Run the demo seeder
make seed-demo

# Or run directly
go run cmd/seed-demo/main.go
```

## Demo Credentials

**Demo Admin:**
- Email: `demo@hamsaya.af`
- Password: `Demo123!`

**Demo Users:**
- Email pattern: `firstname.lastname@demo.af`
- Password: `Demo123!` (same for all demo users)

Examples:
- `alice.wilson@demo.af`
- `bob.anderson@demo.af`
- `carol.martinez@demo.af`

## When to Use

Use this seeder when you need:

1. **Demo Environment**: Setting up a demo for clients or stakeholders
2. **Testing**: Need realistic data for testing features
3. **Development**: Want a populated database for frontend development
4. **Training**: Setting up environments for training purposes
5. **Presentations**: Need a fully functional system to showcase

## Difference from Basic Seed

| Feature | Basic Seed | Demo Seed |
|---------|-----------|-----------|
| Users | 10 | 15 |
| Businesses | 8 | 10 |
| Categories | 10 | 12 |
| Posts | 11 | 20 |
| Comments | ❌ | ✅ 10+ |
| Likes | ❌ | ✅ 30+ |
| Poll Options | ❌ | ✅ 20 |
| Poll Votes | ❌ | ✅ Multiple |
| Event Interests | ❌ | ✅ 40+ |
| User Relationships | ❌ | ✅ 20+ |
| Reports | ❌ | ✅ 14 |
| Idempotent | ❌ | ✅ |

## Data Distribution

### Posts by Type:
- 25% FEED posts (community updates)
- 25% EVENT posts (gatherings, workshops)
- 25% SELL posts (marketplace items)
- 25% PULL posts (polls and surveys)

### Engagement:
- Average 5 likes per post
- 1-2 comments per popular post
- 8 users per event (interested + going)
- 20+ follow relationships

### Businesses:
- Mix of active businesses (all active in demo)
- Different provinces and districts
- Various industries (tech, food, crafts, etc.)

## Resetting Demo Data

To completely reset and re-seed demo data:

```bash
# Option 1: Reset entire database
docker-compose down -v
docker-compose up -d postgres redis minio
make migrate-up
make seed-demo

# Option 2: Manual cleanup (advanced)
# Connect to database and truncate specific tables
docker exec -it hamsaya-postgres psql -U hamsaya -d hamsaya_db
# Then run TRUNCATE commands
```

## Extending Demo Data

To add more demo data:

1. **Add Users**: Extend the `users` array in `seedDemoUsers()`
2. **Add Posts**: Extend the `posts` array in `seedDemoPosts()`
3. **Add Comments**: Extend the `comments` array in `seedDemoComments()`
4. **Add Polls**: Extend the `pollOptions` array in `seedDemoPolls()`

## Troubleshooting

**Issue**: Seeder fails with "user already exists"
- **Solution**: This is normal if running multiple times. The seeder will skip existing users.

**Issue**: Some data is missing
- **Solution**: Ensure migrations are up to date: `make migrate-up`

**Issue**: Polls have no options
- **Solution**: Check that poll creation succeeded before running seeder again

## Production Warning

⚠️ **Never run seeders in production!**
- Seeders are for development and demo only
- They create accounts with known passwords
- Use proper user registration in production
