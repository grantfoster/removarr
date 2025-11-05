#!/bin/bash

# Create an admin user for testing
# Usage: ./scripts/create-admin-user.sh [username] [password]

set -e

USERNAME="${1:-admin}"
PASSWORD="${2:-admin}"

echo "Creating admin user: $USERNAME"

# Hash the password
HASH=$(go run scripts/hash-password.go "$PASSWORD" 2>/dev/null || echo "")

if [ -z "$HASH" ]; then
    echo "❌ Failed to hash password. Please install Go or use a different method."
    exit 1
fi

# Insert user into database
docker exec removarr-test-db psql -U removarr -d removarr -c "
INSERT INTO users (username, password_hash, is_admin, is_active)
VALUES ('$USERNAME', '$HASH', true, true)
ON CONFLICT (username) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    is_admin = true,
    is_active = true;
" 2>&1 | grep -v "Password" || true

echo "✅ Admin user created:"
echo "   Username: $USERNAME"
echo "   Password: $PASSWORD"
echo ""
echo "   You can now use Basic Auth in Swagger with these credentials!"

