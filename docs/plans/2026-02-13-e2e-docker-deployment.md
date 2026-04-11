# DOSRouter E2E Testing, Docker Validation & Deployment

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Comprehensive E2E test coverage, Docker install/uninstall validation (10 cases), and automated deployment pipeline for DOSRouter.

**Architecture:** Three-phase approach: (1) Expand E2E tests to cover error scenarios, edge cases, and the recent fixes (504 timeout, settlement retry, large payload handling), (2) Build Docker-based installation testing covering npm global, DOSRouter plugin, upgrade/downgrade, and cleanup scenarios, (3) Automate deployment with pre-publish validation and version management.

**Tech Stack:** TypeScript, tsx test runner, Docker, npm, DOSRouter CLI, bash scripting

---

## Task 1: E2E Test Expansion

**Goal:** Add 10+ new E2E test cases covering error handling, edge cases, and recent bug fixes.

**Files:**

- Modify: `test/test-e2e.ts`
- Run: `npx tsx test/test-e2e.ts`

### Step 1: Add test for 413 Payload Too Large (150KB limit)

**Code to add after existing tests (before cleanup section):**

```typescript
// Test 8: 413 Payload Too Large — message array exceeds 150KB
allPassed =
  (await test(
    "413 error for oversized request (>150KB)",
    async (p) => {
      const largeMessage = "x".repeat(160 * 1024); // 160KB
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          messages: [{ role: "user", content: largeMessage }],
          max_tokens: 10,
        }),
      });
      if (res.status !== 413) {
        const text = await res.text();
        throw new Error(`Expected 413, got ${res.status}: ${text.slice(0, 200)}`);
      }
      const body = await res.json();
      if (!body.error?.message?.includes("exceeds maximum"))
        throw new Error("Missing size limit error message");
      console.log(`(payload=${Math.round(largeMessage.length / 1024)}KB, status=413) `);
    },
    proxy,
  )) && allPassed;
```

### Step 2: Add test for 400 Bad Request (malformed JSON)

```typescript
// Test 9: 400 Bad Request — malformed JSON
allPassed =
  (await test(
    "400 error for malformed JSON",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{invalid json}",
      });
      if (res.status !== 400) throw new Error(`Expected 400, got ${res.status}`);
      const body = await res.json();
      if (!body.error) throw new Error("Missing error object");
    },
    proxy,
  )) && allPassed;
```

### Step 3: Add test for 400 Bad Request (missing required fields)

```typescript
// Test 10: 400 Bad Request — missing messages field
allPassed =
  (await test(
    "400 error for missing messages field",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          max_tokens: 10,
          // missing messages
        }),
      });
      if (res.status !== 400) throw new Error(`Expected 400, got ${res.status}`);
      const body = await res.json();
      if (!body.error?.message?.includes("messages"))
        throw new Error("Error should mention missing messages");
    },
    proxy,
  )) && allPassed;
```

### Step 4: Add test for large message array (200 messages limit)

```typescript
// Test 11: 400 error for too many messages (>200)
allPassed =
  (await test(
    "400 error for message array exceeding 200 items",
    async (p) => {
      const messages = Array(201)
        .fill(null)
        .map((_, i) => ({ role: i % 2 === 0 ? "user" : "assistant", content: "test" }));
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          messages,
          max_tokens: 10,
        }),
      });
      if (res.status !== 400) {
        const text = await res.text();
        throw new Error(`Expected 400, got ${res.status}: ${text.slice(0, 200)}`);
      }
      const body = await res.json();
      if (!body.error?.message?.includes("200"))
        throw new Error("Error should mention message limit");
      console.log(`(messages=${messages.length}, status=400) `);
    },
    proxy,
  )) && allPassed;
```

### Step 5: Add test for invalid model name

```typescript
// Test 12: Model fallback — invalid model should fail gracefully
allPassed =
  (await test(
    "Invalid model returns clear error",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "invalid/nonexistent-model",
          messages: [{ role: "user", content: "test" }],
          max_tokens: 10,
        }),
      });
      if (res.status !== 400) {
        const text = await res.text();
        throw new Error(`Expected 400, got ${res.status}: ${text.slice(0, 200)}`);
      }
      const body = await res.json();
      if (!body.error) throw new Error("Missing error object");
    },
    proxy,
  )) && allPassed;
```

### Step 6: Add test for concurrent requests (stress test)

```typescript
// Test 13: Concurrent requests — send 5 parallel requests
allPassed =
  (await test(
    "Concurrent requests (5 parallel)",
    async (p) => {
      const makeRequest = () =>
        fetch(`${p.baseUrl}/v1/chat/completions`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            model: "deepseek/deepseek-chat",
            messages: [{ role: "user", content: `Test ${Math.random()}` }],
            max_tokens: 5,
          }),
        });

      const start = Date.now();
      const results = await Promise.all([
        makeRequest(),
        makeRequest(),
        makeRequest(),
        makeRequest(),
        makeRequest(),
      ]);
      const elapsed = Date.now() - start;

      const allSucceeded = results.every((r) => r.status === 200);
      if (!allSucceeded) {
        const statuses = results.map((r) => r.status).join(", ");
        throw new Error(`Not all requests succeeded: ${statuses}`);
      }

      console.log(`(5 requests in ${elapsed}ms, avg=${Math.round(elapsed / 5)}ms) `);
    },
    proxy,
  )) && allPassed;
```

### Step 7: Add test for negative max_tokens (should be rejected)

```typescript
// Test 14: Negative max_tokens should be rejected
allPassed =
  (await test(
    "400 error for negative max_tokens",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          messages: [{ role: "user", content: "test" }],
          max_tokens: -100,
        }),
      });
      if (res.status !== 400) throw new Error(`Expected 400, got ${res.status}`);
      const body = await res.json();
      if (!body.error) throw new Error("Missing error object");
    },
    proxy,
  )) && allPassed;
```

### Step 8: Add test for empty messages array

```typescript
// Test 15: Empty messages array should be rejected
allPassed =
  (await test(
    "400 error for empty messages array",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          messages: [],
          max_tokens: 10,
        }),
      });
      if (res.status !== 400) throw new Error(`Expected 400, got ${res.status}`);
    },
    proxy,
  )) && allPassed;
```

### Step 9: Add test for streaming with large response (token counting)

```typescript
// Test 16: Streaming large response — verify token counting
allPassed =
  (await test(
    "Streaming with large output (token counting)",
    async (p) => {
      const res = await fetch(`${p.baseUrl}/v1/chat/completions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          model: "deepseek/deepseek-chat",
          messages: [
            {
              role: "user",
              content: "Write a 50-word story about a robot. Be concise.",
            },
          ],
          max_tokens: 100,
          stream: true,
        }),
      });
      if (res.status !== 200) throw new Error(`Expected 200, got ${res.status}`);

      const text = await res.text();
      const lines = text.split("\n").filter((l) => l.startsWith("data: "));
      const hasDone = lines.some((l) => l === "data: [DONE]");
      if (!hasDone) throw new Error("Missing [DONE] marker");

      let fullContent = "";
      for (const line of lines.filter((l) => l !== "data: [DONE]")) {
        try {
          const parsed = JSON.parse(line.slice(6));
          const delta = parsed.choices?.[0]?.delta?.content;
          if (delta) fullContent += delta;
        } catch {
          // skip
        }
      }

      const wordCount = fullContent.trim().split(/\s+/).length;
      console.log(`(words=${wordCount}, chunks=${lines.length - 1}) `);
      if (wordCount < 10) throw new Error(`Response too short: ${wordCount} words`);
    },
    proxy,
  )) && allPassed;
```

### Step 10: Add test for balance check (verify wallet has funds)

```typescript
// Test 17: Balance check before test (ensure wallet is funded)
allPassed =
  (await test(
    "Wallet has sufficient balance",
    async (p) => {
      if (!p.balanceMonitor) throw new Error("Balance monitor not available");
      const balance = await p.balanceMonitor.checkBalance();
      if (balance.isEmpty) throw new Error("Wallet is empty - please fund it");
      console.log(`(balance=$${balance.balanceUSD.toFixed(2)}) `);
    },
    proxy,
  )) && allPassed;
```

### Step 11: Run expanded E2E tests

Run:

```bash
DOSROUTER_WALLET_KEY=0x... npx tsx test/test-e2e.ts
```

Expected output:

```
=== DOSRouter e2e tests ===

Starting proxy...
Proxy ready on port 8405
  Health check ... (wallet: 0xABC...) PASS
  Non-streaming request (deepseek/deepseek-chat) ... (response: "4") PASS
  Streaming request (google/gemini-2.5-flash) ... (heartbeat=true, done=true, content="Hello") PASS
  Smart routing: simple query (dos/auto → should pick cheap model) ... PASS
  Smart routing: streaming (dos/auto, stream=true) ... PASS
  Dedup: identical request returns cached response ... PASS
  404 for unknown path ... PASS
  413 error for oversized request (>150KB) ... (payload=160KB, status=413) PASS
  400 error for malformed JSON ... PASS
  400 error for missing messages field ... PASS
  400 error for message array exceeding 200 items ... (messages=201, status=400) PASS
  Invalid model returns clear error ... PASS
  Concurrent requests (5 parallel) ... (5 requests in 2500ms, avg=500ms) PASS
  400 error for negative max_tokens ... PASS
  400 error for empty messages array ... PASS
  Streaming with large output (token counting) ... (words=52, chunks=15) PASS
  Wallet has sufficient balance ... (balance=$5.23) PASS

=== ALL TESTS PASSED ===
```

### Step 12: Commit E2E test expansion

```bash
git add test/test-e2e.ts
git commit -m "test: expand E2E coverage with 10 new test cases

- Add 413 Payload Too Large test (150KB limit)
- Add 400 Bad Request tests (malformed JSON, missing fields)
- Add message array limit test (200 messages)
- Add invalid model error handling test
- Add concurrent request stress test (5 parallel)
- Add negative max_tokens validation test
- Add empty messages array validation test
- Add streaming large response test with token counting
- Add wallet balance check test

Covers recent bug fixes: 504 timeout prevention, settlement retry,
large payload truncation."
```

---

## Task 2: Docker Install/Uninstall Tests (10 Cases)

**Goal:** Validate DOSRouter installation, upgrade, uninstall across different methods and environments.

**Files:**

- Create: `test/docker-install-tests.sh`
- Create: `test/Dockerfile.install-test`
- Modify: `test/run-docker-test.sh`

### Step 1: Create Dockerfile for installation testing

**Create `test/Dockerfile.install-test`:**

```dockerfile
FROM node:22-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Create test user
RUN useradd -m -s /bin/bash testuser

# Set up environment
USER testuser
WORKDIR /home/testuser

# Initialize npm config
RUN npm config set prefix ~/.npm-global
ENV PATH="/home/testuser/.npm-global/bin:$PATH"

CMD ["/bin/bash"]
```

### Step 2: Create bash test script with 10 test cases

**Create `test/docker-install-tests.sh`:**

```bash
#!/bin/bash
set -e

PASS=0
FAIL=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test runner
test_case() {
  local name=$1
  local fn=$2

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Test: $name"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if $fn; then
    echo -e "${GREEN}✓ PASS${NC}"
    ((PASS++))
  else
    echo -e "${RED}✗ FAIL${NC}"
    ((FAIL++))
  fi
}

# Test 1: Fresh npm global installation
test_fresh_install() {
  echo "Installing @dos/dosrouter globally..."
  npm install -g @dos/dosrouter@latest

  echo "Verifying dosrouter command exists..."
  which dosrouter || return 1

  echo "Checking version..."
  dosrouter --version || return 1

  echo "Verifying package is in npm global list..."
  npm list -g @dos/dosrouter || return 1

  return 0
}

# Test 2: Uninstall verification
test_uninstall() {
  echo "Uninstalling @dos/dosrouter..."
  npm uninstall -g @dos/dosrouter

  echo "Verifying dosrouter command is gone..."
  if which dosrouter 2>/dev/null; then
    echo "ERROR: dosrouter command still exists after uninstall"
    return 1
  fi

  echo "Verifying package is not in npm global list..."
  if npm list -g @dos/dosrouter 2>/dev/null; then
    echo "ERROR: package still in npm list after uninstall"
    return 1
  fi

  return 0
}

# Test 3: Reinstall after uninstall
test_reinstall() {
  echo "Reinstalling @dos/dosrouter..."
  npm install -g @dos/dosrouter@latest

  echo "Verifying reinstall works..."
  dosrouter --version || return 1

  return 0
}

# Test 4: Installation as DOSRouter plugin (if DOSRouter available)
test_dosrouter_plugin_install() {
  echo "Installing DOSRouter..."
  npm install -g dosrouter@latest || {
    echo "DOSRouter not available, skipping test"
    return 0
  }

  echo "Installing DOSRouter as DOSRouter plugin..."
  dosrouter plugins install @dos/dosrouter || return 1

  echo "Verifying plugin is listed..."
  dosrouter plugins list | grep -q "dosrouter" || return 1

  return 0
}

# Test 5: DOSRouter plugin uninstall
test_dosrouter_plugin_uninstall() {
  if ! which dosrouter 2>/dev/null; then
    echo "DOSRouter not available, skipping test"
    return 0
  fi

  echo "Uninstalling DOSRouter plugin..."
  dosrouter plugins uninstall dosrouter || return 1

  echo "Verifying plugin is removed..."
  if dosrouter plugins list 2>/dev/null | grep -q "dosrouter"; then
    echo "ERROR: plugin still listed after uninstall"
    return 1
  fi

  return 0
}

# Test 6: Upgrade from previous version
test_upgrade() {
  echo "Installing older version (0.8.25)..."
  npm install -g @dos/dosrouter@0.8.25

  echo "Verifying old version..."
  local old_version=$(dosrouter --version)
  echo "Installed: $old_version"

  echo "Upgrading to latest..."
  npm install -g @dos/dosrouter@latest

  echo "Verifying upgrade..."
  local new_version=$(dosrouter --version)
  echo "Upgraded to: $new_version"

  if [ "$old_version" = "$new_version" ]; then
    echo "ERROR: version did not change after upgrade"
    return 1
  fi

  return 0
}

# Test 7: Installation with custom wallet key
test_custom_wallet() {
  echo "Setting custom wallet key..."
  export DOSROUTER_WALLET_KEY="0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

  echo "Installing with wallet key..."
  npm install -g @dos/dosrouter@latest

  echo "Verifying installation..."
  dosrouter --version || return 1

  unset DOSROUTER_WALLET_KEY
  return 0
}

# Test 8: Verify package files exist
test_package_files() {
  echo "Installing @dos/dosrouter..."
  npm install -g @dos/dosrouter@latest

  echo "Finding package installation directory..."
  local pkg_dir=$(npm root -g)/@dos/dosrouter

  echo "Checking for required files..."
  [ -f "$pkg_dir/dist/index.js" ] || { echo "Missing dist/index.js"; return 1; }
  [ -f "$pkg_dir/dist/cli.js" ] || { echo "Missing dist/cli.js"; return 1; }
  [ -f "$pkg_dir/package.json" ] || { echo "Missing package.json"; return 1; }
  [ -f "$pkg_dir/dosrouter.plugin.json" ] || { echo "Missing dosrouter.plugin.json"; return 1; }

  echo "All required files present"
  return 0
}

# Test 9: Version command accuracy
test_version_command() {
  echo "Installing @dos/dosrouter..."
  npm install -g @dos/dosrouter@latest

  echo "Running version command..."
  local cli_version=$(dosrouter --version)

  echo "Reading package.json version..."
  local pkg_dir=$(npm root -g)/@dos/dosrouter
  local pkg_version=$(node -p "require('$pkg_dir/package.json').version")

  echo "CLI version: $cli_version"
  echo "Package version: $pkg_version"

  if [ "$cli_version" != "$pkg_version" ]; then
    echo "ERROR: version mismatch"
    return 1
  fi

  return 0
}

# Test 10: Full cleanup verification
test_full_cleanup() {
  echo "Installing @dos/dosrouter..."
  npm install -g @dos/dosrouter@latest

  echo "Finding all DOSRouter files..."
  local pkg_dir=$(npm root -g)/@dos/dosrouter
  local bin_link=$(which dosrouter)

  echo "Package dir: $pkg_dir"
  echo "Binary link: $bin_link"

  echo "Uninstalling..."
  npm uninstall -g @dos/dosrouter

  echo "Verifying complete cleanup..."
  if [ -d "$pkg_dir" ]; then
    echo "ERROR: package directory still exists: $pkg_dir"
    return 1
  fi

  if [ -f "$bin_link" ] || [ -L "$bin_link" ]; then
    echo "ERROR: binary link still exists: $bin_link"
    return 1
  fi

  echo "Complete cleanup verified"
  return 0
}

# Run all tests
echo "╔════════════════════════════════════════════════════════╗"
echo "║   DOSRouter Docker Installation Test Suite          ║"
echo "╚════════════════════════════════════════════════════════╝"

test_case "1. Fresh npm global installation" test_fresh_install
test_case "2. Uninstall verification" test_uninstall
test_case "3. Reinstall after uninstall" test_reinstall
test_case "4. DOSRouter plugin installation" test_dosrouter_plugin_install
test_case "5. DOSRouter plugin uninstall" test_dosrouter_plugin_uninstall
test_case "6. Upgrade from previous version" test_upgrade
test_case "7. Installation with custom wallet" test_custom_wallet
test_case "8. Package files verification" test_package_files
test_case "9. Version command accuracy" test_version_command
test_case "10. Full cleanup verification" test_full_cleanup

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary: $PASS passed, $FAIL failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

[ $FAIL -eq 0 ] && exit 0 || exit 1
```

### Step 3: Make test script executable

```bash
chmod +x test/docker-install-tests.sh
```

### Step 4: Update run-docker-test.sh to include installation tests

**Modify `test/run-docker-test.sh`:**

```bash
#!/bin/bash
set -e

cd "$(dirname "$0")/.."

echo "🐳 Building Docker test environment for installation tests..."
docker build -f test/Dockerfile.install-test -t dosrouter-install-test .

echo ""
echo "🧪 Running installation test suite (10 test cases)..."
docker run --rm \
    -v "$(pwd)/test/docker-install-tests.sh:/test.sh:ro" \
    dosrouter-install-test \
    bash -c "cp /test.sh /tmp/test.sh && chmod +x /tmp/test.sh && /tmp/test.sh"

echo ""
echo "✅ Docker installation tests completed successfully!"
```

### Step 5: Run Docker installation tests

Run:

```bash
./test/run-docker-test.sh
```

Expected output:

```
🐳 Building Docker test environment for installation tests...
...
🧪 Running installation test suite (10 test cases)...

╔════════════════════════════════════════════════════════╗
║   DOSRouter Docker Installation Test Suite          ║
╚════════════════════════════════════════════════════════╝

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Test: 1. Fresh npm global installation
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Installing @dos/dosrouter globally...
Verifying dosrouter command exists...
Checking version...
0.8.30
✓ PASS

[... 9 more tests ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary: 10 passed, 0 failed
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Docker installation tests completed successfully!
```

### Step 6: Commit Docker installation tests

```bash
git add test/Dockerfile.install-test test/docker-install-tests.sh test/run-docker-test.sh
git commit -m "test: add Docker-based installation testing (10 test cases)

Test coverage:
- Fresh npm global installation
- Uninstall verification
- Reinstall after uninstall
- DOSRouter plugin install/uninstall
- Upgrade from previous version
- Custom wallet key installation
- Package files verification
- Version command accuracy
- Full cleanup verification

Validates installation, upgrade, uninstall workflows in isolated Docker
environment."
```

---

## Task 3: Deployment Automation

**Goal:** Automate pre-publish validation, version bumping, npm publish, and GitHub release creation.

**Files:**

- Create: `scripts/deploy.sh`
- Modify: `package.json` (add deploy script)

### Step 1: Create deployment script

**Create `scripts/deploy.sh`:**

```bash
#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  DOSRouter Deployment Pipeline${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Step 1: Check git status (must be clean)
echo ""
echo -e "${YELLOW}1. Checking git status...${NC}"
if [[ -n $(git status --porcelain) ]]; then
  echo -e "${RED}ERROR: Working directory is not clean. Commit or stash changes first.${NC}"
  exit 1
fi
echo "✓ Working directory is clean"

# Step 2: Check we're on main branch
echo ""
echo -e "${YELLOW}2. Checking branch...${NC}"
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
  echo -e "${RED}ERROR: Must be on main branch (currently on $BRANCH)${NC}"
  exit 1
fi
echo "✓ On main branch"

# Step 3: Pull latest changes
echo ""
echo -e "${YELLOW}3. Pulling latest changes...${NC}"
git pull origin main
echo "✓ Up to date with origin/main"

# Step 4: Install dependencies
echo ""
echo -e "${YELLOW}4. Installing dependencies...${NC}"
npm ci
echo "✓ Dependencies installed"

# Step 5: Run typecheck
echo ""
echo -e "${YELLOW}5. Running typecheck...${NC}"
npm run typecheck
echo "✓ Typecheck passed"

# Step 6: Run build
echo ""
echo -e "${YELLOW}6. Building project...${NC}"
npm run build
echo "✓ Build successful"

# Step 7: Run tests
echo ""
echo -e "${YELLOW}7. Running tests...${NC}"

# Check if wallet key is set
if [ -z "$DOSROUTER_WALLET_KEY" ]; then
  echo -e "${YELLOW}WARNING: DOSROUTER_WALLET_KEY not set. Skipping E2E tests.${NC}"
  echo "Set DOSROUTER_WALLET_KEY to run E2E tests during deployment."
else
  echo "Running E2E tests..."
  npx tsx test/test-e2e.ts
  echo "✓ E2E tests passed"
fi

# Step 8: Get version bump type
echo ""
echo -e "${YELLOW}8. Version bump${NC}"
echo "Current version: $(node -p "require('./package.json').version")"
echo ""
echo "Select version bump type:"
echo "  1) patch (0.8.30 → 0.8.31)"
echo "  2) minor (0.8.30 → 0.9.0)"
echo "  3) major (0.8.30 → 1.0.0)"
echo "  4) custom"
read -p "Enter choice (1-4): " VERSION_CHOICE

case $VERSION_CHOICE in
  1)
    VERSION_TYPE="patch"
    ;;
  2)
    VERSION_TYPE="minor"
    ;;
  3)
    VERSION_TYPE="major"
    ;;
  4)
    read -p "Enter custom version (e.g., 1.0.0-beta.1): " CUSTOM_VERSION
    npm version "$CUSTOM_VERSION" --no-git-tag-version
    NEW_VERSION="$CUSTOM_VERSION"
    ;;
  *)
    echo -e "${RED}Invalid choice${NC}"
    exit 1
    ;;
esac

# Bump version if not custom
if [ -n "$VERSION_TYPE" ]; then
  NEW_VERSION=$(npm version "$VERSION_TYPE" --no-git-tag-version)
  NEW_VERSION=${NEW_VERSION#v} # Remove leading 'v'
fi

echo "✓ Version bumped to $NEW_VERSION"

# Step 9: Update version in src/version.ts
echo ""
echo -e "${YELLOW}9. Updating version in source files...${NC}"
cat > src/version.ts <<EOF
/**
 * DOSRouter version
 * Auto-generated during deployment
 */
export const VERSION = "$NEW_VERSION";
EOF
echo "✓ Version updated in src/version.ts"

# Step 10: Rebuild with new version
echo ""
echo -e "${YELLOW}10. Rebuilding with new version...${NC}"
npm run build
echo "✓ Rebuild successful"

# Step 11: Commit version bump
echo ""
echo -e "${YELLOW}11. Committing version bump...${NC}"
git add package.json package-lock.json src/version.ts
git commit -m "$NEW_VERSION"
echo "✓ Version bump committed"

# Step 12: Create git tag
echo ""
echo -e "${YELLOW}12. Creating git tag...${NC}"
git tag -a "v$NEW_VERSION" -m "Release v$NEW_VERSION"
echo "✓ Tag v$NEW_VERSION created"

# Step 13: Confirm publish
echo ""
echo -e "${YELLOW}13. Ready to publish${NC}"
echo ""
echo "Package: @dos/dosrouter"
echo "Version: $NEW_VERSION"
echo "Registry: https://registry.npmjs.org"
echo ""
read -p "Publish to npm? (y/N): " CONFIRM_PUBLISH

if [ "$CONFIRM_PUBLISH" != "y" ] && [ "$CONFIRM_PUBLISH" != "Y" ]; then
  echo -e "${YELLOW}Publish cancelled. Version was bumped but not published.${NC}"
  echo "To publish later, run: npm publish"
  exit 0
fi

# Step 14: Publish to npm
echo ""
echo -e "${YELLOW}14. Publishing to npm...${NC}"
npm publish --access public
echo "✓ Published to npm"

# Step 15: Push to GitHub
echo ""
echo -e "${YELLOW}15. Pushing to GitHub...${NC}"
git push origin main
git push origin "v$NEW_VERSION"
echo "✓ Pushed to GitHub"

# Step 16: Create GitHub release
echo ""
echo -e "${YELLOW}16. Creating GitHub release...${NC}"
if command -v gh &> /dev/null; then
  gh release create "v$NEW_VERSION" \
    --title "v$NEW_VERSION" \
    --notes "Release v$NEW_VERSION" \
    --generate-notes
  echo "✓ GitHub release created"
else
  echo -e "${YELLOW}WARNING: gh CLI not found. Skipping GitHub release creation.${NC}"
  echo "Create release manually at: https://github.com/DOSAI/DOSRouter/releases/new"
fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Deployment Complete! 🎉${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Package: @dos/dosrouter@$NEW_VERSION"
echo "npm: https://www.npmjs.com/package/@dos/dosrouter"
echo "GitHub: https://github.com/DOSAI/DOSRouter/releases/tag/v$NEW_VERSION"
echo ""
```

### Step 2: Make deployment script executable

```bash
chmod +x scripts/deploy.sh
```

### Step 3: Add deploy command to package.json

**Modify `package.json` scripts section:**

```json
{
  "scripts": {
    "build": "tsup",
    "dev": "tsup --watch",
    "typecheck": "tsc --noEmit",
    "lint": "eslint src/",
    "format": "prettier --write .",
    "format:check": "prettier --check .",
    "test:resilience:errors": "npx tsx test/resilience-errors.ts",
    "test:resilience:stability": "DURATION_MINUTES=5 npx tsx test/resilience-stability.ts",
    "test:resilience:stability:full": "DURATION_MINUTES=240 npx tsx test/resilience-stability.ts",
    "test:resilience:lifecycle": "npx tsx test/resilience-lifecycle.ts",
    "test:resilience:quick": "npm run test:resilience:errors && npm run test:resilience:lifecycle",
    "test:resilience:full": "npm run test:resilience:errors && npm run test:resilience:lifecycle && npm run test:resilience:stability:full",
    "test:e2e:tool-ids": "npx tsx test/e2e-tool-id-sanitization.ts",
    "test:docker:install": "./test/run-docker-test.sh",
    "deploy": "./scripts/deploy.sh"
  }
}
```

### Step 4: Test deployment script (dry run)

**Before running the full deployment, test the validation steps:**

```bash
# Test git status check
git status

# Test typecheck
npm run typecheck

# Test build
npm run build

# Test E2E (if wallet key set)
DOSROUTER_WALLET_KEY=0x... npx tsx test/test-e2e.ts
```

### Step 5: Document deployment process

**Create `docs/deployment.md`:**

````markdown
# DOSRouter Deployment Guide

## Prerequisites

1. **npm account with publish access** to `@dos/dosrouter`
2. **GitHub CLI (`gh`)** installed (optional, for automated release creation)
3. **Funded wallet** for E2E tests (optional, but recommended)

## Deployment Process

### Option 1: Automated Deployment (Recommended)

```bash
# Set wallet key for E2E tests (optional)
export DOSROUTER_WALLET_KEY=0x...

# Run deployment script
npm run deploy
```
````

The script will:

1. ✓ Check git status is clean
2. ✓ Verify on main branch
3. ✓ Pull latest changes
4. ✓ Install dependencies
5. ✓ Run typecheck
6. ✓ Build project
7. ✓ Run E2E tests (if wallet key set)
8. ✓ Prompt for version bump type
9. ✓ Update version in package.json and src/version.ts
10. ✓ Rebuild with new version
11. ✓ Commit version bump
12. ✓ Create git tag
13. ✓ Publish to npm
14. ✓ Push to GitHub
15. ✓ Create GitHub release

### Option 2: Manual Deployment

```bash
# 1. Update version
npm version patch  # or minor, or major

# 2. Update src/version.ts
echo 'export const VERSION = "0.8.31";' > src/version.ts

# 3. Build
npm run build

# 4. Commit
git add package.json package-lock.json src/version.ts
git commit -m "0.8.31"
git tag -a v0.8.31 -m "Release v0.8.31"

# 5. Publish
npm publish --access public

# 6. Push
git push origin main
git push origin v0.8.31

# 7. Create GitHub release
gh release create v0.8.31 --title "v0.8.31" --generate-notes
```

## Version Bump Types

- **patch**: Bug fixes, minor changes (0.8.30 → 0.8.31)
- **minor**: New features, non-breaking changes (0.8.30 → 0.9.0)
- **major**: Breaking changes (0.8.30 → 1.0.0)
- **custom**: Pre-release versions (0.8.30 → 1.0.0-beta.1)

## Post-Deployment Verification

1. Check npm package: https://www.npmjs.com/package/@dos/dosrouter
2. Verify installation: `npm install -g @dos/dosrouter@latest`
3. Test version: `dosrouter --version`
4. Check GitHub release: https://github.com/DOSAI/DOSRouter/releases

## Rollback

If deployment fails:

```bash
# Delete tag locally and remotely
git tag -d v0.8.31
git push origin :refs/tags/v0.8.31

# Revert version commit
git revert HEAD
git push origin main

# Unpublish from npm (within 72 hours)
npm unpublish @dos/dosrouter@0.8.31
```

## Troubleshooting

### "Working directory is not clean"

Commit or stash changes before deploying:

```bash
git status
git add .
git commit -m "feat: ..."
```

### "Must be on main branch"

Switch to main:

```bash
git checkout main
```

### E2E tests fail

Set wallet key:

```bash
export DOSROUTER_WALLET_KEY=0x...
```

Or skip E2E tests (not recommended):

```bash
# Edit scripts/deploy.sh and comment out E2E test section
```

````

### Step 6: Run deployment script (test mode)

**Test the deployment script without publishing:**

```bash
# Comment out the npm publish and git push steps in scripts/deploy.sh
# Then run:
npm run deploy

# Select patch version bump
# Review all steps
# Decline publish when prompted
````

Expected output:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  DOSRouter Deployment Pipeline
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Checking git status...
✓ Working directory is clean

2. Checking branch...
✓ On main branch

3. Pulling latest changes...
✓ Up to date with origin/main

4. Installing dependencies...
✓ Dependencies installed

5. Running typecheck...
✓ Typecheck passed

6. Building project...
✓ Build successful

7. Running tests...
✓ E2E tests passed

8. Version bump
Current version: 0.8.30

Select version bump type:
  1) patch (0.8.30 → 0.8.31)
  2) minor (0.8.30 → 0.9.0)
  3) major (0.8.30 → 1.0.0)
  4) custom
Enter choice (1-4): 1
✓ Version bumped to 0.8.31

9. Updating version in source files...
✓ Version updated in src/version.ts

10. Rebuilding with new version...
✓ Rebuild successful

11. Committing version bump...
✓ Version bump committed

12. Creating git tag...
✓ Tag v0.8.31 created

13. Ready to publish

Package: @dos/dosrouter
Version: 0.8.31
Registry: https://registry.npmjs.org

Publish to npm? (y/N): N
Publish cancelled. Version was bumped but not published.
To publish later, run: npm publish
```

### Step 7: Commit deployment automation

```bash
git add scripts/deploy.sh package.json docs/deployment.md
git commit -m "chore: add automated deployment pipeline

- Add deployment script with pre-publish validation
- Version bump with interactive selection
- Automatic git tag creation
- npm publish with confirmation
- GitHub release creation (requires gh CLI)
- Add deployment documentation

Usage: npm run deploy"
```

---

## Execution Handoff

Plan complete and saved to `docs/plans/2026-02-13-e2e-docker-deployment.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach would you prefer, Your Majesty?**
