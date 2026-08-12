package engine

import (
	"sync"
	"velyxora/packages/types"
)

// FeeType defines standard categories of platform fees
type FeeType string

const (
	FeeTrading        FeeType = "TRADING"
	FeeWithdrawal     FeeType = "WITHDRAWAL"
	FeeDeposit        FeeType = "DEPOSIT"
	FeeTransfer       FeeType = "TRANSFER"
	FeeAdministrative FeeType = "ADMINISTRATIVE"
)

// VIPConfig maps volume requirements to fee discounts
type VIPConfig struct {
	Level      int     `json:"level"`
	MinVolume  float64 `json:"min_volume"` // USD
	MakerRate  float64 `json:"maker_rate"`
	TakerRate  float64 `json:"taker_rate"`
}

// FeesEngine calculates Maker, Taker, Withdrawal, Transfer, and Referral fees dynamically
type FeesEngine struct {
	mu               sync.RWMutex
	userVolume30D    map[string]float64 // userID -> USD volume
	referrals        map[string]string  // userID -> referrerID
	zeroFeeMarkets   map[string]bool    // symbol -> is zero fee
	feeOverrides     map[string]float64 // userID -> specific fee rate overrides
	accumulatedFees  map[string]float64 // asset -> accumulated revenue
	defaultMakerRate float64
	defaultTakerRate float64
	referralRate     float64            // e.g. 0.20 (20% of trading fee goes to referrer)
	vipTiers         []VIPConfig

	// Dynamic native token discount support
	preferences      map[string]*types.UserPreferences
	assetPrices      map[string]float64 // asset -> price in USDT
	riskEngine       *RiskEngine
}

// NewFeesEngine creates an upgraded Enterprise commission and revenue manager
func NewFeesEngine(maker, taker float64) *FeesEngine {
	// Initialize default prices (e.g., VLX = 2.5 USDT, USDT = 1.0)
	prices := map[string]float64{
		"VLX":  2.5,
		"USDT": 1.0,
	}

	return &FeesEngine{
		userVolume30D:    make(map[string]float64),
		referrals:        make(map[string]string),
		zeroFeeMarkets:   make(map[string]bool),
		feeOverrides:     make(map[string]float64),
		accumulatedFees:  make(map[string]float64),
		defaultMakerRate: maker,
		defaultTakerRate: taker,
		referralRate:     0.20, // 20% standard referral reward
		vipTiers: []VIPConfig{
			{Level: 0, MinVolume: 0.0, MakerRate: maker, TakerRate: taker},
			{Level: 1, MinVolume: 100000.0, MakerRate: 0.0005, TakerRate: 0.0015},
			{Level: 2, MinVolume: 1000000.0, MakerRate: 0.0002, TakerRate: 0.0010},
			{Level: 3, MinVolume: 10000000.0, MakerRate: 0.0000, TakerRate: 0.0005},
		},
		preferences: make(map[string]*types.UserPreferences),
		assetPrices: prices,
	}
}

// SetUserPreference registers or overrides user preference settings
func (fe *FeesEngine) SetUserPreference(userID string, pref *types.UserPreferences) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.preferences[userID] = pref
}

// SetAssetPrice sets/updates an asset conversion price
func (fe *FeesEngine) SetAssetPrice(asset string, price float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.assetPrices[asset] = price
}

// SetRiskEngine links risk engine for checking balances
func (fe *FeesEngine) SetRiskEngine(re *RiskEngine) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.riskEngine = re
}

// RegisterReferral links a user to a referrer
func (fe *FeesEngine) RegisterReferral(userID, referrerID string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.referrals[userID] = referrerID
}

// SetZeroFeeMarket activates promotional zero-fee tarrifs
func (fe *FeesEngine) SetZeroFeeMarket(symbol string, zeroFee bool) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.zeroFeeMarkets[symbol] = zeroFee
}

// SetUserFeeOverride configures customized fee override rate parameters
func (fe *FeesEngine) SetUserFeeOverride(userID string, rate float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.feeOverrides[userID] = rate
}

// SetUserVolume sets user 30D trading volume
func (fe *FeesEngine) SetUserVolume(userID string, volume float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.userVolume30D[userID] = volume
}

// GetUserVIPLevel computes active VIP status
func (fe *FeesEngine) GetUserVIPLevel(userID string) int {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	vol := fe.userVolume30D[userID]
	highestLevel := 0
	for _, tier := range fe.vipTiers {
		if vol >= tier.MinVolume && tier.Level > highestLevel {
			highestLevel = tier.Level
		}
	}
	return highestLevel
}

// GetFeeRates calculates rates with VIP tiers, overrides, and market exemptions
func (fe *FeesEngine) GetFeeRates(userID string) (makerRate, takerRate float64) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	// 1. User specific override check
	if override, ok := fe.feeOverrides[userID]; ok {
		return override, override
	}

	// 2. VIP tiers check
	vol := fe.userVolume30D[userID]
	maker := fe.defaultMakerRate
	taker := fe.defaultTakerRate

	for _, tier := range fe.vipTiers {
		if vol >= tier.MinVolume {
			maker = tier.MakerRate
			taker = tier.TakerRate
		}
	}

	return maker, taker
}

// CalculateFee returns fee amounts, referrer commission cuts, and final platform revenue
func (fe *FeesEngine) CalculateFee(userID string, price, quantity float64, isMaker bool) float64 {
	makerRate, takerRate := fe.GetFeeRates(userID)
	rate := takerRate
	if isMaker {
		rate = makerRate
	}
	return price * quantity * rate
}

// ProcessCommission splits transaction fees into affiliate referral commissions and platform revenues
func (fe *FeesEngine) ProcessCommission(userID, asset string, totalFee float64) (referrerID string, commission, revenue float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	refID, hasReferrer := fe.referrals[userID]
	if hasReferrer && refID != "" {
		commission = totalFee * fe.referralRate
		revenue = totalFee - commission
		fe.accumulatedFees[asset] += revenue
		return refID, commission, revenue
	}

	fe.accumulatedFees[asset] += totalFee
	return "", 0.0, totalFee
}

// GetAccumulatedRevenue retrieves collected fees by asset
func (fe *FeesEngine) GetAccumulatedRevenue(asset string) float64 {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.accumulatedFees[asset]
}

// CalculateFeeForUser checks user preference and available VLX balance, and applies 25% discount if applicable
func (fe *FeesEngine) CalculateFeeForUser(userID string, symbol string, rawFeeAmount float64, isMaker bool) (feeAsset string, finalFee float64, discounted bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	// Default fee asset is the quote asset or the native asset of the trade.
	// We'll determine the default fee asset from the symbol (assumed to be of form BASE-QUOTE).
	defaultAsset := "USDT"
	parts := splitSymbol(symbol)
	if len(parts) == 2 {
		defaultAsset = parts[1] // Quote asset
	}

	// 1. Check if user wants to pay fees in VLX
	pref, hasPref := fe.preferences[userID]
	if !hasPref || pref == nil || !pref.PayFeesInVLX {
		return defaultAsset, rawFeeAmount, false
	}

	// 2. Query raw fee asset price and VLX price to perform conversion
	vlxPrice, ok := fe.assetPrices["VLX"]
	if !ok || vlxPrice <= 0 {
		vlxPrice = 2.5 // fallback default
	}

	rawAssetPrice, ok := fe.assetPrices[defaultAsset]
	if !ok || rawAssetPrice <= 0 {
		// If defaultAsset is USDT, price is 1.0. For others, default to 1.0 if not found
		rawAssetPrice = 1.0
	}

	// Convert raw fee amount in rawAsset to VLX equivalent
	// rawFeeAmountInUSDT = rawFeeAmount * rawAssetPrice
	// rawFeeAmountInVLX = rawFeeAmountInUSDT / vlxPrice
	rawFeeInUSDT := rawFeeAmount * rawAssetPrice
	rawFeeInVLX := rawFeeInUSDT / vlxPrice

	// Apply 25% discount
	discountedVLXFee := rawFeeInVLX * 0.75

	// 3. Verify user has sufficient VLX balance
	if fe.riskEngine != nil {
		availableVLX := fe.riskEngine.GetAvailableBalance(userID, "VLX")
		if availableVLX >= discountedVLXFee {
			return "VLX", discountedVLXFee, true
		}
	}

	// Fallback to standard asset
	return defaultAsset, rawFeeAmount, false
}

// Helper to split trading symbol
func splitSymbol(symbol string) []string {
	// e.g. "BTC-USDT" or "BTC/USDT"
	var parts []string
	if len(symbol) > 0 {
		for _, sep := range []string{"-", "/"} {
			if pos := int(0); pos < len(symbol) {
				idx := -1
				for i := 0; i < len(symbol); i++ {
					if symbol[i:i+1] == sep {
						idx = i
						break
					}
				}
				if idx != -1 {
					parts = []string{symbol[:idx], symbol[idx+1:]}
					break
				}
			}
		}
	}
	if len(parts) != 2 {
		parts = []string{symbol, "USDT"}
	}
	return parts
}
