#!/bin/bash
# Script to prepare the Homebrew tap repository locally

set -e

echo "Preparing Homebrew tap for omasword..."

# Create tap directory
TAP_DIR="homebrew-omasword"
if [ -d "$TAP_DIR" ]; then
    echo "Error: $TAP_DIR directory already exists!"
    echo "Please remove it or choose a different location."
    exit 1
fi

echo "Creating tap directory: $TAP_DIR"
mkdir -p "$TAP_DIR"

# Copy formula
echo "Copying formula..."
cp -r Formula "$TAP_DIR/"

# Update formula with current SHA256
echo "Calculating SHA256 for latest release..."
cd "$TAP_DIR"

# For now, we'll prepare it with empty SHA256
# You'll need to update this after creating the release
cat > Formula/omasword.rb << 'EOF'
class Omasword < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/badenpiland/omasword"
  url "https://github.com/badenpiland/omasword/archive/refs/tags/v1.11.0.tar.gz"
  sha256 "" # This will need to be updated with the actual SHA256 of the release tarball
  license "GPL-2.0-or-later"
  head "https://github.com/badenpiland/omasword.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X omasword/internal/version.BuildNumber=#{version}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/omasword"
  end

  test do
    # Test that the binary was installed and can run
    assert_match version.to_s, shell_output("#{bin}/omasword --version")
  end
end
EOF

# Create README for the tap
cat > README.md << 'EOF'
# Homebrew Tap for omasword

This is a Homebrew tap for [omasword](https://github.com/badenpiland/omasword), a terminal-based Bible application.

## Installation

```bash
brew tap badenpiland/omasword
brew install omasword
```

## Updating

```bash
brew update
brew upgrade omasword
```

## Formula

The formula is maintained in the main omasword repository and synchronized here for Homebrew distribution.

## License

This tap is licensed under the same license as omasword (GPL-2.0-or-later).
EOF

# Create .gitignore
cat > .gitignore << 'EOF'
.DS_Store
*.swp
*~
EOF

# Initialize git repository
echo "Initializing git repository..."
git init
git add .
git commit -m "Initial tap setup with omasword formula"

cd ..

echo ""
echo "✅ Homebrew tap prepared successfully!"
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Create a new GitHub repository named 'homebrew-omasword'"
echo "   Go to: https://github.com/new"
echo "   - Repository name: homebrew-omasword"
echo "   - Make it public"
echo "   - Don't initialize with README, .gitignore, or license"
echo ""
echo "2. Push the tap to GitHub:"
echo "   cd homebrew-omasword"
echo "   git remote add origin git@github.com:badenpiland/homebrew-omasword.git"
echo "   git push -u origin main"
echo ""
echo "3. Update the formula with the correct SHA256:"
echo "   - Download the release tarball from GitHub"
echo "   - Run: sha256sum v1.11.0.tar.gz"
echo "   - Update the sha256 line in Formula/omasword.rb"
echo "   - Commit and push the change"
echo ""
echo "4. Test the tap:"
echo "   brew tap badenpiland/omasword"
echo "   brew install omasword"
echo ""
echo "The tap directory is ready at: ./homebrew-omasword"