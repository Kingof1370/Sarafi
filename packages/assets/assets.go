package assets

import (
	"context"
	"errors"
	"sync"
)

// AssetType defines the class of asset
type AssetType string

const (
	TypeNativeCoin   AssetType = "NATIVE"
	TypeToken        AssetType = "TOKEN"
	TypeStablecoin   AssetType = "STABLECOIN"
	TypeWrappedAsset AssetType = "WRAPPED"
)

// Asset represents a supported cryptocurrency on the exchange
type Asset struct {
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Type        AssetType `json:"type"`
	Precision   int       `json:"precision"`
	BaseNetwork string    `json:"base_network"`
	IsActive    bool      `json:"is_active"`
}

// AssetMetadata stores supplemental attributes for the asset
type AssetMetadata struct {
	Symbol           string `json:"symbol"`
	ContractAddress  string `json:"contract_address,omitempty"`
	LogoURL          string `json:"logo_url,omitempty"`
	Description      string `json:"description,omitempty"`
	Website          string `json:"website,omitempty"`
	ExplorerURL      string `json:"explorer_url,omitempty"`
	TotalSupply      string `json:"total_supply,omitempty"`
	CirculatingPrice float64 `json:"circulating_price,omitempty"`
}

// AssetPermission defines strict action permissions for operations on an asset
type AssetPermission struct {
	Symbol             string  `json:"symbol"`
	CanDeposit         bool    `json:"can_deposit"`
	CanWithdraw        bool    `json:"can_withdraw"`
	CanTrade           bool    `json:"can_trade"`
	MinDepositAmount   float64 `json:"min_deposit_amount"`
	MinWithdrawAmount  float64 `json:"min_withdraw_amount"`
	MaxDailyWithdrawal float64 `json:"max_daily_withdrawal"`
	WithdrawalFee      float64 `json:"withdrawal_fee"`
}

// AssetRegistry manages standard in-memory and database-synchronized asset profiles
type AssetRegistry struct {
	mu          sync.RWMutex
	assets      map[string]*Asset
	metadata    map[string]*AssetMetadata
	permissions map[string]*AssetPermission
}

// NewAssetRegistry instantiates a clean asset storage module
func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{
		assets:      make(map[string]*Asset),
		metadata:    make(map[string]*AssetMetadata),
		permissions: make(map[string]*AssetPermission),
	}
}

// RegisterAsset adds a newly supported asset, metadata, and permissions to the exchange registries
func (ar *AssetRegistry) RegisterAsset(asset *Asset, meta *AssetMetadata, perm *AssetPermission) error {
	if asset == nil || asset.Symbol == "" {
		return errors.New("invalid asset symbol details")
	}

	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.assets[asset.Symbol] = asset

	if meta != nil {
		ar.metadata[asset.Symbol] = meta
	} else {
		ar.metadata[asset.Symbol] = &AssetMetadata{Symbol: asset.Symbol}
	}

	if perm != nil {
		ar.permissions[asset.Symbol] = perm
	} else {
		ar.permissions[asset.Symbol] = &AssetPermission{
			Symbol:             asset.Symbol,
			CanDeposit:         true,
			CanWithdraw:        true,
			CanTrade:           true,
			MinDepositAmount:   0.0001,
			MinWithdrawAmount:  0.0002,
			MaxDailyWithdrawal: 10.0,
			WithdrawalFee:      0.0001,
		}
	}

	return nil
}

// GetAsset fetches an asset configuration profile
func (ar *AssetRegistry) GetAsset(symbol string) (*Asset, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	ast, exists := ar.assets[symbol]
	if !exists {
		return nil, errors.New("asset not found in exchange registry")
	}
	return ast, nil
}

// GetMetadata retrieves supplemental asset attributes
func (ar *AssetRegistry) GetMetadata(symbol string) (*AssetMetadata, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	meta, exists := ar.metadata[symbol]
	if !exists {
		return nil, errors.New("asset metadata not found")
	}
	return meta, nil
}

// GetPermissions reads strict action boundaries of the asset
func (ar *AssetRegistry) GetPermissions(symbol string) (*AssetPermission, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	perm, exists := ar.permissions[symbol]
	if !exists {
		return nil, errors.New("asset permissions configuration not found")
	}
	return perm, nil
}

// ListAssets returns all registered assets on the platform
func (ar *AssetRegistry) ListAssets(ctx context.Context) []*Asset {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	list := make([]*Asset, 0, len(ar.assets))
	for _, ast := range ar.assets {
		list = append(list, ast)
	}
	return list
}
