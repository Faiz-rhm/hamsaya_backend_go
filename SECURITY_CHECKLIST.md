# Security Hardening Checklist

**Last Updated:** 2025-01-24

## Backend API Security

- [ ] Admin routes protected with RequireAdmin middleware
- [ ] Admin actions logged to audit trail
- [ ] CORS wildcard rejected in production
- [ ] CORS origins use HTTPS only in production
- [ ] OAuth tokens validated for audience claim
- [ ] OAuth tokens validated for issuer and expiration
- [ ] Rate limiting on auth endpoints (5 req/15min)
- [ ] Rate limiting on report endpoints (10 req/hour)
- [ ] Rate limiting on post creation (20 req/hour)
- [ ] Rate limiting on comment creation (30 req/hour)
- [ ] Rate limiting on password reset (3 req/hour)
- [ ] SQL injection patterns fixed in admin repository
- [ ] Parameterized queries used throughout
- [ ] Input sanitization on all user inputs
- [ ] Security headers applied globally
- [ ] HTTPS enforced in production
- [ ] JWT secrets are 64+ characters
- [ ] Database passwords are 32+ characters
- [ ] MinIO credentials rotated
- [ ] Firebase credentials secured
- [ ] Environment variables validated on startup

## Dashboard Security

- [ ] Tokens stored in httpOnly cookies (not localStorage)
- [ ] CSRF tokens implemented and validated
- [ ] CSP headers configured
- [ ] Input sanitization on all forms
- [ ] XSS protection on user-generated content
- [ ] Secure API URL configuration
- [ ] HTTPS enforced for all requests
- [ ] Session timeout implemented
- [ ] Logout clears all tokens and cookies

## Mobile App Security

- [ ] Tokens stored in flutter_secure_storage
- [ ] Certificate pinning implemented
- [ ] Jailbreak/root detection implemented
- [ ] Code obfuscation enabled for production builds
- [ ] ProGuard rules configured for Android
- [ ] API keys not hardcoded in source
- [ ] Deep link validation implemented

## Database Security

- [ ] All migrations applied
- [ ] Indexes created for performance
- [ ] Type-specific constraints enforced
- [ ] Foreign keys with CASCADE rules
- [ ] Unique constraints on email/phone
- [ ] PostGIS optimizations applied
- [ ] Regular backups scheduled
- [ ] Backup restoration tested
- [ ] Connection pooling configured
- [ ] Database not publicly accessible

## Infrastructure Security

- [ ] SSL/TLS certificates installed
- [ ] SSL certificates auto-renewal configured
- [ ] Firewall rules configured
- [ ] Database not publicly accessible
- [ ] Redis not publicly accessible
- [ ] MinIO not publicly accessible
- [ ] Server hardening completed
- [ ] Log aggregation configured
- [ ] Monitoring and alerting set up

## Compliance

- [ ] Privacy policy published
- [ ] Terms of service published
- [ ] Data retention policy implemented
- [ ] User data export functionality
- [ ] User data deletion functionality
- [ ] Cookie consent implemented

## Testing

- [ ] Penetration testing completed
- [ ] Security audit performed
- [ ] Dependency vulnerabilities scanned
- [ ] OWASP Top 10 validated
- [ ] Authentication flows tested
- [ ] Authorization flows tested
- [ ] Rate limiting tested
- [ ] SQL injection tested
- [ ] XSS tested
- [ ] CSRF tested

## References

- See `docs/IMPLEMENTATION_GUIDES.md` for implementation details
- See `DEPLOYMENT_CHECKLIST.md` for deployment steps
