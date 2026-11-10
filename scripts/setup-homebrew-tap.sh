#!/bin/bash
# Script to help set up a Homebrew tap repository

set -e

echo "This script will help you set up a Homebrew tap for omasword"
echo "Prerequisites:"
echo "  - You need a GitHub account"
echo "  - You need to create a repository named 'homebrew-omasword'"
echo ""
read -p "Have you created the 'homebrew-omasword' repository? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Please create the repository first at: https://github.com/new"
    echo "Name it: homebrew-omasword"
    exit 1
fi

# Clone the tap repository
read -p "Enter your GitHub username: " GITHUB_USERNAME
echo "Cloning tap repository..."
git clone "git@github.com:${GITHUB_USERNAME}/homebrew-omasword.git" /tmp/homebrew-omasword

# Copy formula
echo "Copying formula..."
cp -r Formula /tmp/homebrew-omasword/

# Update formula with current SHA256
echo "Updating formula with latest release info..."
cd /tmp/homebrew-omasword
../scripts/update-formula.sh

# Create README for the tap
cat > README.md << 'EOF'
# Homebrew Tap for omasword

This is a Homebrew tap for [omasword](https://github.com/badenpiland/omasword), a terminal-based Bible application.

## Installation

```bash
brew tap badenpiland/omasword
brew install omasword
```

## Formula

The formula is maintained in the main omasword repository and synchronized here for Homebrew distribution.
EOF

# Commit and push
echo "Setting up git..."
git add .
git commit -m "Initial tap setup with omasword formula"
git push origin main

echo ""
echo "✅ Homebrew tap setup complete!"
echo ""
echo "Users can now install omasword with:"
echo "  brew tap ${GITHUB_USERNAME}/omasword"
echo "  brew install omasword"
echo ""
echo "Don't forget to update the README.md in the main repository with the correct tap name!"

# Cleanup
rm -rf /tmp/homebrew-omasword