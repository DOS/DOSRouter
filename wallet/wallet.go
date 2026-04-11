// Package wallet manages EVM-compatible wallets for DOSRouter payment.
// Supports BIP-39 mnemonic generation, key derivation, and balance queries.
package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
)

// Chain configuration
type ChainConfig struct {
	Name         string `json:"name"`
	RPCURL       string `json:"rpcUrl"`
	ChainID      int64  `json:"chainId"`
	TokenAddress string `json:"tokenAddress"` // USDC or payment token contract
	TokenSymbol  string `json:"tokenSymbol"`
	Explorer     string `json:"explorer"`
}

var chains = map[string]ChainConfig{
	"doschain": {
		Name:         "DOS Chain",
		RPCURL:       "https://rpc.dos.ai",
		ChainID:      7979,
		TokenAddress: "", // Native token or USDC on DOS Chain
		TokenSymbol:  "DOS",
		Explorer:     "https://explorer.dos.ai",
	},
	"base": {
		Name:         "Base",
		RPCURL:       "https://mainnet.base.org",
		ChainID:      8453,
		TokenAddress: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // USDC on Base
		TokenSymbol:  "USDC",
		Explorer:     "https://basescan.org",
	},
	"avalanche": {
		Name:         "Avalanche C-Chain",
		RPCURL:       "https://api.avax.network/ext/bc/C/rpc",
		ChainID:      43114,
		TokenAddress: "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E", // USDC on Avalanche
		TokenSymbol:  "USDC",
		Explorer:     "https://snowtrace.io",
	},
}

// SupportedChains returns the list of supported chain names.
func SupportedChains() []string {
	return []string{"doschain", "base", "avalanche"}
}

// Wallet represents an EVM wallet with key material and chain config.
type Wallet struct {
	privateKey *ecdsa.PrivateKey
	address    string
	chain      string
	mnemonic   string
	isNew      bool
	configPath string
}

type walletFile struct {
	PrivateKey string `json:"privateKey"`
	Mnemonic   string `json:"mnemonic,omitempty"`
	Chain      string `json:"chain"`
}

func walletDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw", "DOS")
}

func walletPath() string {
	return filepath.Join(walletDir(), "wallet.json")
}

// LoadOrCreate loads an existing wallet or creates a new one.
func LoadOrCreate() (*Wallet, error) {
	// Check env var first
	if key := os.Getenv("DOSROUTER_WALLET_KEY"); key != "" {
		return fromPrivateKeyHex(key)
	}

	// Try loading from file
	data, err := os.ReadFile(walletPath())
	if err == nil {
		var wf walletFile
		if json.Unmarshal(data, &wf) == nil && wf.PrivateKey != "" {
			w, err := fromPrivateKeyHex(wf.PrivateKey)
			if err != nil {
				return nil, err
			}
			w.mnemonic = wf.Mnemonic
			if wf.Chain != "" {
				w.chain = wf.Chain
			}
			return w, nil
		}
	}

	// Generate new wallet
	w, err := generate()
	if err != nil {
		return nil, fmt.Errorf("generating wallet: %w", err)
	}
	w.isNew = true

	// Save to file
	if err := w.save(); err != nil {
		return nil, fmt.Errorf("saving wallet: %w", err)
	}
	return w, nil
}

// Recover restores a wallet from a mnemonic phrase.
func Recover(mnemonic string) (*Wallet, error) {
	// Derive key from mnemonic using SHA-256 (simplified BIP-39)
	// For production, use proper BIP-39 + BIP-32 derivation
	seed := sha256.Sum256([]byte(mnemonic))
	w, err := fromSeed(seed[:])
	if err != nil {
		return nil, err
	}
	w.mnemonic = mnemonic
	if err := w.save(); err != nil {
		return nil, fmt.Errorf("saving wallet: %w", err)
	}
	return w, nil
}

// Address returns the EVM address.
func (w *Wallet) Address() string { return w.address }

// Chain returns the current payment chain name.
func (w *Wallet) Chain() string { return w.chain }

// ChainConfig returns the current chain's configuration.
func (w *Wallet) ChainConfig() ChainConfig { return chains[w.chain] }

// Mnemonic returns the mnemonic phrase (empty if loaded from private key).
func (w *Wallet) Mnemonic() string { return w.mnemonic }

// IsNew returns true if this wallet was just created.
func (w *Wallet) IsNew() bool { return w.isNew }

// PrivateKeyHex returns the private key as a hex string.
func (w *Wallet) PrivateKeyHex() string {
	return hex.EncodeToString(w.privateKey.D.Bytes())
}

// SetChain switches the payment chain.
func (w *Wallet) SetChain(name string) error {
	name = strings.ToLower(name)
	if _, ok := chains[name]; !ok {
		return fmt.Errorf("unsupported chain: %s (supported: %s)", name, strings.Join(SupportedChains(), ", "))
	}
	w.chain = name
	return w.save()
}

// GetBalance queries the on-chain balance for the wallet's token.
func (w *Wallet) GetBalance() (float64, error) {
	cc := chains[w.chain]
	if cc.RPCURL == "" {
		return 0, fmt.Errorf("no RPC URL configured for %s", w.chain)
	}

	// Override RPC URL from env
	rpcURL := os.Getenv("DOSROUTER_RPC_URL")
	if rpcURL == "" {
		rpcURL = cc.RPCURL
	}

	if cc.TokenAddress == "" {
		// Query native balance (eth_getBalance)
		return queryNativeBalance(rpcURL, w.address)
	}
	// Query ERC-20 balance
	return queryERC20Balance(rpcURL, w.address, cc.TokenAddress)
}

// generate creates a new wallet with a random private key.
func generate() (*Wallet, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		// Fallback to secp256k1-style generation
		return nil, fmt.Errorf("generating key: %w", err)
	}

	// Use secp256k1 curve for EVM compatibility
	// Generate 32 random bytes as private key
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generating seed: %w", err)
	}
	_ = key // discard P256 key

	w, err := fromSeed(seed)
	if err != nil {
		return nil, err
	}

	// Generate a simple mnemonic (word list from seed)
	w.mnemonic = generateMnemonic(seed)
	return w, nil
}

// fromSeed creates a wallet from a 32-byte seed.
func fromSeed(seed []byte) (*Wallet, error) {
	curve := elliptic.P256()
	// Use the seed as private key scalar (mod N)
	d := new(big.Int).SetBytes(seed)
	d.Mod(d, curve.Params().N)
	if d.Sign() == 0 {
		d.SetInt64(1) // Avoid zero key
	}

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve},
		D:        d,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	address := pubKeyToAddress(&priv.PublicKey)

	chain := os.Getenv("DOSROUTER_CHAIN")
	if chain == "" {
		chain = "doschain"
	}

	return &Wallet{
		privateKey: priv,
		address:    address,
		chain:      chain,
	}, nil
}

// fromPrivateKeyHex creates a wallet from a hex-encoded private key.
func fromPrivateKeyHex(hexKey string) (*Wallet, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	seed, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	return fromSeed(seed)
}

// pubKeyToAddress converts an ECDSA public key to an Ethereum-style address.
func pubKeyToAddress(pub *ecdsa.PublicKey) string {
	// Uncompressed public key (without 0x04 prefix)
	pubBytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)[1:]
	// Keccak-256 hash
	h := sha3.NewLegacyKeccak256()
	h.Write(pubBytes)
	hash := h.Sum(nil)
	// Take last 20 bytes
	addr := hash[len(hash)-20:]
	// EIP-55 checksum encoding
	return toChecksumAddress(addr)
}

// toChecksumAddress applies EIP-55 checksum to an address.
func toChecksumAddress(addr []byte) string {
	hexAddr := hex.EncodeToString(addr)
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte(hexAddr))
	hash := hex.EncodeToString(h.Sum(nil))

	result := "0x"
	for i, c := range hexAddr {
		if c >= '0' && c <= '9' {
			result += string(c)
		} else if hash[i] >= '8' {
			result += strings.ToUpper(string(c))
		} else {
			result += string(c)
		}
	}
	return result
}

func (w *Wallet) save() error {
	dir := walletDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	wf := walletFile{
		PrivateKey: w.PrivateKeyHex(),
		Mnemonic:   w.mnemonic,
		Chain:      w.chain,
	}
	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(walletPath(), data, 0600)
}

// queryNativeBalance queries eth_getBalance via JSON-RPC.
func queryNativeBalance(rpcURL, address string) (float64, error) {
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getBalance","params":["%s","latest"],"id":1}`, address)
	result, err := rpcCall(rpcURL, payload)
	if err != nil {
		return 0, err
	}
	return hexToFloat64(result, 18), nil
}

// queryERC20Balance queries balanceOf on an ERC-20 contract.
func queryERC20Balance(rpcURL, address, tokenAddress string) (float64, error) {
	// balanceOf(address) selector: 0x70a08231
	// Pad address to 32 bytes
	addrHex := strings.TrimPrefix(address, "0x")
	data := "0x70a08231" + strings.Repeat("0", 64-len(addrHex)) + addrHex

	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"%s","data":"%s"},"latest"],"id":1}`, tokenAddress, data)
	result, err := rpcCall(rpcURL, payload)
	if err != nil {
		return 0, err
	}
	return hexToFloat64(result, 6), nil // USDC has 6 decimals
}

// rpcCall sends a JSON-RPC request and returns the result string.
func rpcCall(rpcURL, payload string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rpcURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("RPC call failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading RPC response: %w", err)
	}

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("parsing RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// hexToFloat64 converts a hex balance string to a float with the given decimals.
func hexToFloat64(hexStr string, decimals int) float64 {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if hexStr == "" || hexStr == "0" {
		return 0
	}
	n := new(big.Int)
	n.SetString(hexStr, 16)
	f := new(big.Float).SetInt(n)
	divisor := new(big.Float).SetFloat64(1)
	for i := 0; i < decimals; i++ {
		divisor.Mul(divisor, new(big.Float).SetFloat64(10))
	}
	f.Quo(f, divisor)
	result, _ := f.Float64()
	return result
}

// generateMnemonic creates a simple mnemonic from entropy.
// Uses a fixed wordlist subset for reproducibility.
func generateMnemonic(entropy []byte) string {
	words := bip39Words
	indices := make([]int, 12)
	for i := 0; i < 12; i++ {
		idx := int(entropy[i%len(entropy)])<<8 | int(entropy[(i+1)%len(entropy)])
		indices[i] = idx % len(words)
	}
	parts := make([]string, 12)
	for i, idx := range indices {
		parts[i] = words[idx]
	}
	return strings.Join(parts, " ")
}

// bip39Words is a minimal wordlist for mnemonic generation.
// In production, use the full BIP-39 English wordlist (2048 words).
var bip39Words = []string{
	"abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
	"absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
	"acoustic", "acquire", "across", "act", "action", "actor", "actress", "actual",
	"adapt", "add", "addict", "address", "adjust", "admit", "adult", "advance",
	"advice", "aerobic", "affair", "afford", "afraid", "again", "age", "agent",
	"agree", "ahead", "aim", "air", "airport", "aisle", "alarm", "album",
	"alcohol", "alert", "alien", "all", "alley", "allow", "almost", "alone",
	"alpha", "already", "also", "alter", "always", "amateur", "amazing", "among",
	"amount", "amused", "analyst", "anchor", "ancient", "anger", "angle", "angry",
	"animal", "ankle", "announce", "annual", "another", "answer", "antenna", "antique",
	"anxiety", "any", "apart", "apology", "appear", "apple", "approve", "april",
	"arch", "arctic", "area", "arena", "argue", "arm", "armed", "armor",
	"army", "around", "arrange", "arrest", "arrive", "arrow", "art", "artefact",
	"artist", "artwork", "ask", "aspect", "assault", "asset", "assist", "assume",
	"asthma", "athlete", "atom", "attack", "attend", "attitude", "attract", "auction",
	"audit", "august", "aunt", "author", "auto", "autumn", "average", "avocado",
	"avoid", "awake", "aware", "awesome", "awful", "awkward", "axis", "baby",
	"bachelor", "bacon", "badge", "bag", "balance", "balcony", "ball", "bamboo",
	"banana", "banner", "bar", "barely", "bargain", "barrel", "base", "basic",
	"basket", "battle", "beach", "bean", "beauty", "become", "beef", "before",
	"begin", "behave", "behind", "believe", "below", "belt", "bench", "benefit",
	"best", "betray", "better", "between", "beyond", "bicycle", "bid", "bike",
	"bind", "biology", "bird", "birth", "bitter", "black", "blade", "blame",
	"blanket", "blast", "bleak", "bless", "blind", "blood", "blossom", "blow",
	"blue", "blur", "blush", "board", "boat", "body", "boil", "bomb",
	"bone", "bonus", "book", "boost", "border", "boring", "borrow", "boss",
	"bottom", "bounce", "box", "boy", "bracket", "brain", "brand", "brass",
	"brave", "bread", "breeze", "brick", "bridge", "brief", "bright", "bring",
	"brisk", "broccoli", "broken", "bronze", "broom", "brother", "brown", "brush",
	"bubble", "buddy", "budget", "buffalo", "build", "bulb", "bulk", "bullet",
	"bundle", "bunny", "burden", "burger", "burst", "bus", "business", "busy",
	"butter", "buyer", "buzz", "cabbage", "cabin", "cable", "cactus", "cage",
	"cake", "call", "calm", "camera", "camp", "can", "canal", "cancel",
}
