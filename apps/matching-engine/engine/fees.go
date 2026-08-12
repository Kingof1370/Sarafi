package engine

import (
	"sync"
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
	safuInsurance    map[string]float64 // asset -> SAFU insurance fund reserve
	defaultMakerRate float64
	defaultTakerRate float64
	referralRate     float64            // e.g. 0.20 (20% of trading fee goes to referrer)
	safuRate         float64            // e.g. 0.10 (10% of trading fee dynamically dedicated to SAFU)
	vipTiers         []VIPConfig
}

// NewFeesEngine creates an upgraded Enterprise commission and revenue manager
func NewFeesEngine(maker, taker float64) *FeesEngine {
	return &FeesEngine{
		userVolume30D:    make(map[string]float64),
		referrals:        make(map[string]string),
		zeroFeeMarkets:   make(map[string]bool),
		feeOverrides:     make(map[string]float64),
		accumulatedFees:  make(map[string]float64),
		safuInsurance:    make(map[string]float64),
		defaultMakerRate: maker,
		defaultTakerRate: taker,
		referralRate:     0.20, // 20% standard referral reward
		safuRate:         0.10, // 10% dynamically goes to SAFU secure wallet reserve
		vipTiers: []VIPConfig{
			{Level: 0, MinVolume: 0.0, MakerRate: maker, TakerRate: taker},
			{Level: 1, MinVolume: 100000.0, MakerRate: 0.0005, TakerRate: 0.0015},
			{Level: 2, MinVolume: 1000000.0, MakerRate: 0.0002, TakerRate: 0.0010},
			{Level: 3, MinVolume: 10000000.0, MakerRate: 0.0000, TakerRate: 0.0005},
		},
	}
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

// ProcessCommission splits transaction fees into affiliate referral commissions, SAFU reserves, and platform revenues
func (fe *FeesEngine) ProcessCommission(userID, asset string, totalFee float64) (referrerID string, commission, revenue float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	// Dedicate a flat rate dynamically to SAFU insurance reserve
	safuCut := totalFee * fe.safuRate
	fe.safuInsurance[asset] += safuCut
	remainingFee := totalFee - safuCut

	refID, hasReferrer := fe.referrals[userID]
	if hasReferrer && refID != "" {
		commission = remainingFee * fe.referralRate
		revenue = remainingFee - commission
		fe.accumulatedFees[asset] += revenue
		return refID, commission, revenue
	}

	fe.accumulatedFees[asset] += remainingFee
	return "", 0.0, remainingFee
}

// GetAccumulatedRevenue retrieves collected fees by asset
func (fe *FeesEngine) GetAccumulatedRevenue(asset string) float64 {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.accumulatedFees[asset]
}

// GetSAFUReserve retrieves the accumulated SAFU reserve balance for a given asset
func (fe *FeesEngine) GetSAFUReserve(asset string) float64 {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.safuInsurance[asset]
}
