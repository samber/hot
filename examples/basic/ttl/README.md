# TTL Cache Example

Demonstrates time-based expiration features in the HOT cache library.

## What it does

- Creates caches with different TTL (Time To Live) settings
- Shows automatic expiration of cached items
- Demonstrates jitter to prevent cache stampedes
- Uses background cleanup with janitor

## Key Features Demonstrated

- **Fixed TTL**: Items expire after a set time period
- **Jitter**: Random expiration times to prevent cache stampedes
- **Custom TTL**: Different expiration times per item
- **Background Cleanup**: Automatic removal of expired items

## Quick Start

```bash
go run ttl.go
```

## Expected Output

```
⏰ TTL Cache Example
====================

📝 Example 1: Basic TTL
------------------------
✅ TTL cache created with 5-minute expiration
✅ Stored session for user:1 (expires at 14:30:15)
✅ Stored session for user:2 (expires at 14:30:15)
🔍 Checking sessions immediately:
✅ Session found: user:1 (expires at 14:30:15)
✅ Session found: user:2 (expires at 14:30:15)

🎲 Example 2: TTL with Jitter
-------------------------------
✅ Jitter cache created with 10-minute TTL ± 2 minutes
✅ Stored config: app:theme = dark
✅ Stored config: app:language = en
✅ Stored config: app:timezone = UTC

⚙️ Example 3: Custom TTL per Item
----------------------------------
✅ Stored items with custom TTLs:
   - short-lived: 30 seconds
   - medium-lived: 5 minutes
   - long-lived: 2 hours
```
