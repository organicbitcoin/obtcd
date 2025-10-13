#!/bin/bash

# OBTC Upstream Rebase Script
# Safely syncs with btcd upstream while preserving OBTC modifications

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
UPSTREAM_REMOTE="upstream"
UPSTREAM_URL="https://github.com/btcsuite/btcd.git"
MAIN_BRANCH="master"
BACKUP_BRANCH="backup-$(date +%Y%m%d-%H%M%S)"

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if we're in a git repository
check_git_repo() {
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_error "Not in a git repository"
        exit 1
    fi
    
    print_success "Git repository detected"
}

# Check current branch
check_current_branch() {
    local current_branch=$(git branch --show-current)
    
    if [ "$current_branch" = "$MAIN_BRANCH" ]; then
        print_error "Cannot rebase on main branch ($MAIN_BRANCH)"
        print_status "Please switch to a development branch first:"
        print_status "  git checkout -b dev/your-feature-branch"
        exit 1
    fi
    
    print_success "Current branch: $current_branch"
    CURRENT_BRANCH="$current_branch"
}

# Check if upstream remote exists
setup_upstream() {
    print_status "Setting up upstream remote..."
    
    if git remote get-url "$UPSTREAM_REMOTE" > /dev/null 2>&1; then
        local existing_url=$(git remote get-url "$UPSTREAM_REMOTE")
        if [ "$existing_url" != "$UPSTREAM_URL" ]; then
            print_warning "Upstream URL mismatch. Updating..."
            git remote set-url "$UPSTREAM_REMOTE" "$UPSTREAM_URL"
        fi
        print_success "Upstream remote already configured"
    else
        print_status "Adding upstream remote..."
        git remote add "$UPSTREAM_REMOTE" "$UPSTREAM_URL"
        print_success "Upstream remote added"
    fi
}

# Fetch latest upstream changes
fetch_upstream() {
    print_status "Fetching upstream changes..."
    git fetch "$UPSTREAM_REMOTE"
    
    local upstream_commits=$(git rev-list --count HEAD..upstream/master)
    print_success "Fetched upstream changes ($upstream_commits new commits)"
    
    if [ "$upstream_commits" -eq 0 ]; then
        print_success "Already up to date with upstream"
        exit 0
    fi
}

# Create backup branch
create_backup() {
    print_status "Creating backup branch: $BACKUP_BRANCH"
    git branch "$BACKUP_BRANCH"
    print_success "Backup created at $BACKUP_BRANCH"
}

# Check for uncommitted changes
check_clean_working_tree() {
    if ! git diff-index --quiet HEAD --; then
        print_error "Working tree has uncommitted changes"
        print_status "Please commit or stash your changes first:"
        print_status "  git add ."
        print_status "  git commit -m 'WIP: save changes before rebase'"
        print_status "  # or"
        print_status "  git stash"
        exit 1
    fi
    
    print_success "Working tree is clean"
}

# Show OBTC-specific files that might conflict
show_obtc_files() {
    print_status "OBTC-specific files to watch for conflicts:"
    echo "  - chaincfg/params_obtc.go"
    echo "  - chaincfg/params_obtc_test.go" 
    echo "  - wire/protocol.go (OBTC network constants)"
    echo "  - scripts/devnet-up.sh"
    echo "  - scripts/rebase-upstream.sh"
    echo "  - obtc_doc/ (entire directory)"
    echo "  - README.md (OBTC sections)"
    echo ""
}

# Perform the rebase
perform_rebase() {
    print_status "Starting rebase onto upstream/master..."
    print_warning "This may cause conflicts that need manual resolution"
    
    show_obtc_files
    
    if git rebase "upstream/$MAIN_BRANCH"; then
        print_success "Rebase completed successfully!"
    else
        print_error "Rebase encountered conflicts"
        print_status "To resolve conflicts:"
        print_status "  1. Edit conflicted files"
        print_status "  2. git add <resolved-files>"
        print_status "  3. git rebase --continue"
        print_status ""
        print_status "To abort rebase:"
        print_status "  git rebase --abort"
        print_status "  git checkout $BACKUP_BRANCH  # restore backup"
        exit 1
    fi
}

# Verify OBTC functionality after rebase
verify_obtc() {
    print_status "Verifying OBTC functionality..."
    
    # Build test
    if ! go build ./...; then
        print_error "Build failed after rebase"
        print_status "This indicates breaking changes in upstream"
        print_status "You may need to update OBTC code to match new btcd APIs"
        return 1
    fi
    
    # Run OBTC tests
    if ! go test ./chaincfg ./wire -run "OBTC"; then
        print_error "OBTC tests failed after rebase"
        print_status "OBTC functionality may be broken"
        return 1
    fi
    
    print_success "OBTC functionality verified"
}

# Show summary
show_summary() {
    local old_commit=$(git rev-parse "$BACKUP_BRANCH")
    local new_commit=$(git rev-parse HEAD)
    local commits_added=$(git rev-list --count "$old_commit..upstream/$MAIN_BRANCH")
    
    print_success "Rebase Summary:"
    echo "  Old commit: $old_commit"
    echo "  New commit: $new_commit"
    echo "  Upstream commits added: $commits_added"
    echo "  Backup branch: $BACKUP_BRANCH"
    echo ""
    print_status "To push updated branch:"
    print_status "  git push --force-with-lease origin $CURRENT_BRANCH"
    echo ""
    print_status "To clean up backup (after verifying everything works):"
    print_status "  git branch -D $BACKUP_BRANCH"
}

# Show help
show_help() {
    echo "OBTC Upstream Rebase Script"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "This script safely rebases your development branch onto the latest"
    echo "btcd upstream master while preserving OBTC-specific modifications."
    echo ""
    echo "Options:"
    echo "  --dry-run    Show what would be done without making changes"
    echo "  --help       Show this help message"
    echo ""
    echo "Prerequisites:"
    echo "  - Must be on a development branch (not master)"
    echo "  - Working tree must be clean (no uncommitted changes)"
    echo "  - OBTC-specific changes should be in separate commits"
    echo ""
    echo "Safety Features:"
    echo "  - Creates backup branch before rebase"
    echo "  - Verifies OBTC functionality after rebase"
    echo "  - Provides clear instructions for conflict resolution"
    echo ""
    echo "Examples:"
    echo "  $0                    # Perform rebase"
    echo "  $0 --dry-run         # Preview what would be done"
    echo ""
}

# Dry run mode
dry_run() {
    print_status "DRY RUN MODE - No changes will be made"
    echo ""
    
    check_git_repo
    check_current_branch
    setup_upstream
    fetch_upstream
    
    local upstream_commits=$(git rev-list --count HEAD..upstream/master)
    print_status "Would rebase $upstream_commits commits from upstream"
    
    show_obtc_files
    
    print_status "Would create backup branch: $BACKUP_BRANCH"
    print_status "Would rebase $CURRENT_BRANCH onto upstream/master"
    print_status "Would verify OBTC functionality"
    
    echo ""
    print_status "To actually perform the rebase, run without --dry-run"
}

# Main function
main() {
    case "${1:-}" in
        "--help"|"-h")
            show_help
            exit 0
            ;;
        "--dry-run")
            dry_run
            exit 0
            ;;
        "")
            # Normal operation
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
    
    print_status "Starting OBTC upstream rebase process..."
    echo ""
    
    check_git_repo
    check_current_branch
    check_clean_working_tree
    setup_upstream
    fetch_upstream
    create_backup
    perform_rebase
    
    if verify_obtc; then
        show_summary
        print_success "Rebase completed successfully!"
    else
        print_warning "Rebase completed but OBTC verification failed"
        print_status "Please check and fix any issues before proceeding"
    fi
}

# Execute main function with all arguments
main "$@"