# Deployment Guide - Coolify on Hetzner VPS

This document explains how to deploy the Places API using Coolify on a Hetzner VPS.

## Overview

The project is now configured for containerized deployment using:

- **Docker** for containerization
- **Coolify** as PaaS for simplified deployment
- **Hetzner Cloud** VPS for hosting
- **GitHub Actions** for CI/CD (versioning only)

## Prerequisites

### 1. Hetzner VPS Setup

1. Create a Hetzner Cloud server:

   - **Minimum specs:** 2 vCPU, 4GB RAM, 40GB SSD
   - **Recommended:** 4 vCPU, 8GB RAM, 80GB SSD
   - **OS:** Ubuntu 22.04 LTS
   - **Network:** Enable IPv4, configure firewall

2. Configure firewall rules:
   ```bash
   # SSH (22), HTTP (80), HTTPS (443), Coolify (8000)
   ufw allow 22
   ufw allow 80
   ufw allow 443
   ufw allow 8000
   ufw enable
   ```

### 2. Coolify Installation

SSH into your Hetzner VPS and install Coolify:

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker (if not already installed)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Log out and back in for Docker group to take effect
exit
# SSH back in

# Install Coolify
curl -fsSL https://cdn.coollabs.io/coolify/install.sh | bash
```

Access Coolify at `http://your-vps-ip:8000` and complete the initial setup.

## Coolify Configuration

### 1. Create a New Project

1. Log into Coolify dashboard
2. Click **"New Project"**
3. Name: `places-api`
4. Description: `Places API - Location-based services`

### 2. Add GitHub Repository

1. In your project, click **"New Resource"**
2. Select **"Git Repository"**
3. Connect your GitHub account
4. Select your `places_api` repository
5. Choose branch: `main` (or your preferred branch)

### 3. Configure Application Settings

**Build Configuration:**

- **Build Pack:** `Dockerfile`
- **Dockerfile:** `./Dockerfile` (default)
- **Port:** `8080`
- **Health Check URL:** `/healthcheck`

**Environment Variables:**
Add the following environment variables in Coolify:

```bash
# Application
GO_ENV=production
GIN_MODE=release
PORT=8080
CONFIG_FILE=./configs/config.yaml

# Database (Supabase)
SUPABASE_URL=your_supabase_project_url
SUPABASE_KEY=your_supabase_service_role_key

# AI Features (Optional)
OPENAI_API_KEY=your_openai_api_key
```

### 4. Domain Configuration (Optional)

1. Point your domain to your Hetzner VPS IP
2. In Coolify, go to your application settings
3. Add your domain in the **"Domains"** section
4. Enable **"Generate SSL Certificate"** for HTTPS

## Deployment Process

### Automatic Deployment

Coolify monitors your GitHub repository and automatically deploys when:

- New commits are pushed to the configured branch
- You manually trigger a deployment from Coolify dashboard

### Manual Deployment

1. Go to Coolify dashboard
2. Navigate to your Places API project
3. Click **"Deploy"** button
4. Monitor the build logs in real-time

### Deployment Status

Check deployment status:

- **Coolify Dashboard:** Real-time build and deployment logs
- **Application Logs:** Available in Coolify under your app
- **Health Check:** `http://your-domain/healthcheck`

## CI/CD with GitHub Actions

The project includes a versioning workflow that:

- Automatically creates version tags when code is merged to main
- Generates GitHub releases with changelogs
- Triggers Coolify deployment automatically

### Commit Message Convention

Use conventional commits for automatic versioning:

```bash
# Patch version (1.0.0 → 1.0.1)
git commit -m "fix: resolve authentication issue"
git commit -m "docs: update API documentation"

# Minor version (1.0.0 → 1.1.0)
git commit -m "feat: add new search endpoint"
git commit -m "feature: implement caching layer"

# Major version (1.0.0 → 2.0.0)
git commit -m "breaking: change API response format"
git commit -m "major: restructure database schema"
```

## Container Management

### Local Testing

Test the Docker container locally:

```bash
# Build the image
docker build -t places-api .

# Run locally
docker run -p 8080:8080 \
  -e SUPABASE_URL="your_url" \
  -e SUPABASE_KEY="your_key" \
  places-api

# Test health check
curl http://localhost:8080/healthcheck
```

### Resource Monitoring

Monitor your application through Coolify:

- **CPU and Memory usage**
- **Network traffic**
- **Container logs**
- **Deployment history**

## Environment-Specific Configuration

### Development

- Use local config file: `./configs/config.yaml`
- Enable debug logging
- Use development database

### Production (Coolify)

- All configuration via environment variables
- Enable production logging
- Use production Supabase instance
- Enable SSL/HTTPS

## Backup and Maintenance

### Database Backups

Since you're using Supabase, backups are handled by Supabase. Configure:

- Regular automated backups
- Point-in-time recovery
- Export capabilities

### Application Updates

1. Push code changes to GitHub
2. Coolify automatically detects and deploys
3. Zero-downtime deployments with health checks
4. Easy rollback to previous versions

### Server Maintenance

1. **OS Updates:** Regular security updates
2. **Docker Updates:** Keep Docker engine updated
3. **Coolify Updates:** Follow Coolify update procedures
4. **Monitoring:** Set up monitoring and alerting

## Troubleshooting

### Build Failures

**Common issues:**

- Go version compatibility (using 1.24.6)
- Missing dependencies in `go.mod`
- Swagger generation errors
- Docker build context issues

**Solutions:**

```bash
# Check build logs in Coolify
# Verify Dockerfile syntax
# Test build locally first
docker build -t places-api-test .
```

### Runtime Issues

**Application won't start:**

1. Check environment variables are set correctly
2. Verify database connectivity (Supabase)
3. Check port configuration (8080)
4. Review application logs in Coolify

**Health check failures:**

1. Verify `/healthcheck` endpoint works locally
2. Check if application is binding to correct port
3. Review firewall rules
4. Check container network configuration

### Performance Issues

**High resource usage:**

1. Monitor logs for errors or infinite loops
2. Check database query performance
3. Review caching implementation
4. Consider scaling up VPS resources

**Slow response times:**

1. Enable application profiling
2. Check database connection pooling
3. Review external API timeouts
4. Consider adding CDN for static assets

## Security Best Practices

### VPS Security

- Regular OS security updates
- SSH key-only authentication
- Fail2ban for intrusion prevention
- Regular security audits

### Application Security

- Environment variables for secrets
- HTTPS only in production
- Rate limiting enabled
- Input validation and sanitization

### Database Security

- Use Supabase RLS (Row Level Security)
- Principle of least privilege
- Regular access reviews
- Monitor for suspicious activity

## Scaling Considerations

### Vertical Scaling (Single Server)

- Upgrade Hetzner VPS specs
- Add more CPU/RAM as needed
- Monitor resource utilization

### Horizontal Scaling (Multiple Servers)

- Use Coolify's clustering features
- Load balancer configuration
- Shared database (Supabase handles this)
- Session management considerations

## Cost Optimization

### Hetzner VPS Pricing

- Start with smaller instances
- Monitor actual resource usage
- Scale up only when needed
- Consider reserved instances for long-term

### Monitoring Costs

- Set up billing alerts
- Regular usage reviews
- Optimize resource allocation
- Consider auto-scaling policies

## Support and Resources

- **Coolify Documentation:** https://coolify.io/docs
- **Hetzner Cloud Docs:** https://docs.hetzner.com/cloud/
- **Docker Documentation:** https://docs.docker.com/
- **This API Documentation:** See SWAGGER.md

## Migration Checklist

- [x] Removed Render-specific files (render.yaml, GitHub Actions)
- [x] Created Dockerfile for containerization
- [x] Added .dockerignore for optimized builds
- [x] Updated documentation for Coolify deployment
- [ ] Test local Docker build
- [ ] Deploy to Coolify staging environment
- [ ] Configure environment variables
- [ ] Set up domain and SSL (optional)
- [ ] Monitor first production deployment
- [ ] Update DNS records (if using custom domain)
