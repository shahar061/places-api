# Deployment Guide

This document explains the CI/CD setup for the Places API project using GitHub Actions and Render.

## Overview

The project uses a two-stage CI/CD pipeline:

1. **Versioning Workflow** - Automatically creates tags when code is merged to main
2. **Deployment Workflow** - Deploys tagged releases to Render

## Workflows

### 1. Version and Tag (`version-and-tag.yml`)

**Trigger:** Push to `main` branch  
**Purpose:** Automatically version and tag releases

**Features:**

- Runs tests and builds the application
- Uses semantic versioning based on commit messages
- Creates GitHub releases with changelogs
- Supports conventional commit prefixes:
  - `breaking:` or `major:` → Major version bump (1.0.0 → 2.0.0)
  - `feat:` or `feature:` or `minor:` → Minor version bump (1.0.0 → 1.1.0)
  - `fix:` or `patch:` → Patch version bump (1.0.0 → 1.0.1)
  - `docs:`, `chore:`, `refactor:`, `style:`, `test:` → Patch version bump

**Skip CI:** Add `[skip ci]` to commit message to skip the workflow

### 2. Deploy to Render (`deploy-to-render.yml`)

**Trigger:** New GitHub release created  
**Purpose:** Deploy the tagged version to Render

**Features:**

- Builds and tests the application
- Generates Swagger documentation
- Deploys to Render using the official action
- Supports manual deployment via workflow dispatch

## Setup Instructions

### 1. GitHub Repository Setup

The workflows are already created in `.github/workflows/`. No additional setup needed in the repo.

### 2. Render Setup

#### A. Create Render Service

1. Go to [Render Dashboard](https://dashboard.render.com)
2. Click "New" → "Web Service"
3. Connect your GitHub repository
4. Use these settings:
   - **Runtime:** Go
   - **Build Command:** `go mod download && go install github.com/swaggo/swag/cmd/swag@latest && make swagger-gen && make build-linux`
   - **Start Command:** `./places_api_unix server`
   - **Plan:** Choose your preferred plan (free, starter, etc.)

Or use the `render.yaml` file in the root directory for Infrastructure as Code deployment.

#### B. Get Service ID and API Key

1. In your Render service dashboard, the Service ID is in the URL: `https://dashboard.render.com/web/srv-xxxxx` (the `srv-xxxxx` part)
2. Get your API key from [Account Settings → API Keys](https://dashboard.render.com/account)

#### C. Set Environment Variables

In your Render service settings, add these environment variables:

- `SUPABASE_URL` - Your Supabase project URL
- `SUPABASE_KEY` - Your Supabase API key
- `OPENAI_API_KEY` - Your OpenAI API key (if using AI features)
- `GO_ENV=production`
- `GIN_MODE=release`

### 3. GitHub Secrets Setup

Add these secrets to your GitHub repository:

1. Go to Repository → Settings → Secrets and variables → Actions
2. Add these repository secrets:
   - `RENDER_SERVICE_ID` - Your Render service ID (srv-xxxxx)
   - `RENDER_API_KEY` - Your Render API key

## Usage

### Automatic Deployment Flow

1. **Make changes** on a feature branch
2. **Create PR** to main branch
3. **Merge PR** - This triggers the versioning workflow
4. **Tag created** - Automatically creates a new version tag
5. **Release created** - GitHub release with changelog
6. **Deployment triggered** - Automatically deploys to Render

### Manual Deployment

You can manually trigger a deployment:

1. Go to GitHub → Actions → "Deploy to Render"
2. Click "Run workflow"
3. Enter the tag you want to deploy
4. Click "Run workflow"

### Commit Message Examples

```bash
# Patch version bump (1.0.0 → 1.0.1)
git commit -m "fix: resolve authentication bug"
git commit -m "docs: update API documentation"

# Minor version bump (1.0.0 → 1.1.0)
git commit -m "feat: add new search endpoint"
git commit -m "feature: implement caching layer"

# Major version bump (1.0.0 → 2.0.0)
git commit -m "breaking: change API response format"
git commit -m "major: restructure database schema"

# Skip CI entirely
git commit -m "docs: fix typo [skip ci]"
```

## Monitoring

### Deployment Status

- **GitHub Actions:** Check the Actions tab for workflow status
- **Render:** Check the Render dashboard for deployment logs
- **Health Check:** The app includes a `/healthcheck` endpoint

### Rollback

If a deployment fails:

1. Check Render logs for errors
2. Fix the issue in a new commit
3. Push to main to trigger a new deployment
4. Or manually deploy a previous working tag

## Troubleshooting

### Build Failures

- Check Go version compatibility (currently using 1.24.6)
- Ensure all dependencies are in `go.mod`
- Check that Swagger generation works locally

### Deployment Failures

- Verify Render service configuration
- Check environment variables are set correctly
- Ensure the binary name matches in render.yaml (`places_api_unix`)
- Check Render service logs

### Version Conflicts

- Ensure you're using conventional commit messages
- Check that the tag doesn't already exist
- Review the commit history for version bumps

## File Structure

```
.github/
  workflows/
    version-and-tag.yml      # Versioning workflow
    deploy-to-render.yml     # Deployment workflow
render.yaml                  # Render service configuration
DEPLOYMENT.md               # This documentation
```

## Next Steps

1. Test the setup by making a small change and pushing to main
2. Monitor the first deployment through GitHub Actions and Render
3. Configure monitoring and alerting as needed
4. Consider adding staging environments for additional testing

## Resources

- [Render Documentation](https://render.com/docs)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)
