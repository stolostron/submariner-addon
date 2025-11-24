# Konflux New Branch Setup

**When:** Creating a new ACM release branch (e.g., release-2.16, release-2.17)

## Overview

This workflow sets up Konflux CI/CD for a new release branch by:
1. Waiting for bot-generated Tekton configs
2. Migrating to the common ACM pipeline
3. Enabling multi-arch builds
4. Passing Enterprise Contract (EC) validation

## Prerequisites

- New release branch exists (e.g., `release-2.16`)
- Konflux bot has created a PR with `.tekton/` configs
- PR is open but failing EC tests (expected - needs migration)
- You have fetched latest remote branches (`git fetch origin`)

## Process

### 1. Review Bot-Generated PR

Bot automatically creates PR like: `Red Hat Konflux update submariner-addon-acm-2XX`

**Expected PR contents:**
- Branch: `konflux-submariner-addon-acm-2XX`
- Files: `.tekton/submariner-addon-acm-2XX-{pull-request,push}.yaml`
- Size: ~636 lines per file (full pipeline spec)
- Build platforms: `linux/x86_64` only
- Status: ❌ EC tests failing (needs migration)

**Check PR:**

```bash
# Find the bot's PR for your release (change 216 to your component number)
gh pr list --search "konflux-submariner-addon-acm-216 in:title" --json number,title,headRefName

# View PR details
gh pr view <PR_NUMBER>

# Note: You'll create a new PR since the bot's branch cannot be pushed to
```

### 2. Setup Working Branch

```bash
NEW_COMPONENT="216"  # Component number (no dots)

# Checkout bot's branch to preserve its commit in git history
git checkout -b "konflux-submariner-addon-acm-$NEW_COMPONENT-$(whoami)" "origin/konflux-submariner-addon-acm-$NEW_COMPONENT"
```

### 3. Copy Working Configs from Previous Release

Instead of migrating bot's configs, copy the working configs from the previous release:

```bash
# Set version numbers
PREV_RELEASE="2.15"
PREV_COMPONENT="215"
NEW_RELEASE="2.16"
NEW_COMPONENT="216"

# Copy working configs from previous release
git checkout "origin/release-$PREV_RELEASE" -- .tekton/

# Rename files to new version
mv ".tekton/submariner-addon-acm-$PREV_COMPONENT-pull-request.yaml" \
   ".tekton/submariner-addon-acm-$NEW_COMPONENT-pull-request.yaml"
mv ".tekton/submariner-addon-acm-$PREV_COMPONENT-push.yaml" \
   ".tekton/submariner-addon-acm-$NEW_COMPONENT-push.yaml"

# Update version numbers in configs
sed -i "s/release-$PREV_RELEASE/release-$NEW_RELEASE/g" .tekton/*.yaml
sed -i "s/acm-$PREV_COMPONENT/acm-$NEW_COMPONENT/g" .tekton/*.yaml
```

### 4. Commit Changes

```bash
# Commit updated configs (on top of bot's commit)
git add .tekton/
git commit -m "Configure Konflux for release-$NEW_RELEASE

Copy working pipeline configs from release-$PREV_RELEASE and adapt for new release.
Configs use common ACM pipeline pattern with multi-arch and hermetic builds.

Signed-off-by: Your Name <your.email@example.com>"
```

**Push and create PR** (manual steps - require authentication):

```bash
# Push branch
git push origin "$(git branch --show-current)"

# Create PR
gh pr create \
  --base "release-$NEW_RELEASE" \
  --title "Add Konflux for $NEW_RELEASE" \
  --body "Configure Konflux for release-$NEW_RELEASE.

Includes bot's original commit plus working pipeline configs from release-$PREV_RELEASE.

- Multi-arch builds enabled
- Hermetic builds for EC compliance
- Source image generation"
```

## Done When

✅ PR created with working pipeline configs

✅ EC validation passing on PR (wait ~5-10 min after pushing):

```bash
gh pr checks <PR_NUMBER>
# Should see all checks passing, including:
# ✓ ec-validation
# ✓ build tests
```

✅ PR merged to release branch:

```bash
RELEASE="2.16"  # Change to your release version
NEW_COMPONENT="216"  # Component number (no dots)

# Check .tekton directory exists on release branch
gh api "repos/stolostron/submariner-addon/contents/.tekton?ref=release-$RELEASE" --jq '.[].name'
# Should show:
# submariner-addon-acm-216-pull-request.yaml
# submariner-addon-acm-216-push.yaml

# Verify file sizes (should be small, not ~636 lines)
gh api "repos/stolostron/submariner-addon/contents/.tekton/submariner-addon-acm-$NEW_COMPONENT-push.yaml?ref=release-$RELEASE" --jq '.content' | base64 -d | wc -l
# Expected: ~60 lines
```

✅ First build on release branch succeeds (wait ~15-30 min after merge, requires `oc login --web https://api.kflux-prd-rh02.0fk9.p1.openshiftapps.com:6443/`):

```bash
NEW_COMPONENT="216"  # Component number (no dots)

# Check recent snapshots for the component
oc get snapshots -n crt-redhat-acm-tenant --sort-by=.metadata.creationTimestamp | grep "submariner-addon-acm-$NEW_COMPONENT" | tail -5

# Get latest snapshot and verify all tests passed
SNAPSHOT=$(oc get snapshots -n crt-redhat-acm-tenant --sort-by=.metadata.creationTimestamp | grep "submariner-addon-acm-$NEW_COMPONENT" | tail -1 | awk '{print $1}')
oc get snapshot $SNAPSHOT -n crt-redhat-acm-tenant -o jsonpath='{.metadata.annotations.test\.appstudio\.openshift\.io/status}' | jq -r '.[] | "\(.scenario): \(.status)"'
# All should show: TestPassed
```