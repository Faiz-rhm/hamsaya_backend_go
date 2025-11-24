#!/bin/bash

# Production secrets validation script

set -e

echo "Validating production secrets..."

# Required secrets
REQUIRED_SECRETS=(
    "DATABASE_URL"
    "REDIS_URL"
    "JWT_SECRET"
    "JWT_REFRESH_SECRET"
    "MINIO_ACCESS_KEY"
    "MINIO_SECRET_KEY"
    "GOOGLE_CLIENT_SECRET"
    "FIREBASE_CREDENTIALS_JSON"
)

# Check each secret
MISSING_SECRETS=()
WEAK_SECRETS=()

for SECRET in "${REQUIRED_SECRETS[@]}"; do
    VALUE="${!SECRET}"

    if [ -z "$VALUE" ]; then
        MISSING_SECRETS+=("$SECRET")
    elif [ ${#VALUE} -lt 32 ]; then
        WEAK_SECRETS+=("$SECRET (length: ${#VALUE}, required: 32+)")
    fi
done

# Report results
if [ ${#MISSING_SECRETS[@]} -gt 0 ]; then
    echo "❌ Missing required secrets:"
    printf '  - %s\n' "${MISSING_SECRETS[@]}"
    exit 1
fi

if [ ${#WEAK_SECRETS[@]} -gt 0 ]; then
    echo "⚠️  Weak secrets detected:"
    printf '  - %s\n' "${WEAK_SECRETS[@]}"
    exit 1
fi

# Validate JWT secret strength
if ! echo "$JWT_SECRET" | grep -qE '[A-Z]' || \
   ! echo "$JWT_SECRET" | grep -qE '[a-z]' || \
   ! echo "$JWT_SECRET" | grep -qE '[0-9]'; then
    echo "❌ JWT_SECRET must contain uppercase, lowercase, and numbers"
    exit 1
fi

# Validate production CORS
if [[ "$CORS_ALLOWED_ORIGINS" == "*" ]] && [[ "$ENVIRONMENT" == "production" ]]; then
    echo "❌ CORS wildcard not allowed in production"
    exit 1
fi

# Validate HTTPS enforcement
if [[ "$ENVIRONMENT" == "production" ]] && [[ "$FORCE_HTTPS" != "true" ]]; then
    echo "❌ HTTPS must be enforced in production"
    exit 1
fi

echo "✅ All secrets validated successfully"
