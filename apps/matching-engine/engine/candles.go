package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
	"velyxora/packages/common"
	"velyxora/packages/database"
)

type Candle struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Interval    string    `json:"interval"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	BaseVolume  float64   `json:"base_volume"`
	QuoteVolume float64   `json:"quote_volume"`
	TradeCount  int       `json:"trade_count"`
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
}

type CandleEngine struct {
	db          *database.DB
	redisClient *common.RedisClient
	intervals   []string
	symbolLocks sync.Map // symbol -> *sync.Mutex
}

func NewCandleEngine(db *database.DB, redisClient *common.RedisClient) *CandleEngine {
	return &CandleEngine{
		db:          db,
		redisClient: redisClient,
		intervals:   []string{"1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w"},
	}
}

func RoundToPrecision(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}

func GetIntervalBoundary(t time.Time, interval string) (time.Time, time.Time) {
	t = t.UTC()
	var openTime, closeTime time.Time

	switch interval {
	case "1m":
		openTime = t.Truncate(time.Minute)
		closeTime = openTime.Add(time.Minute)
	case "5m":
		openTime = t.Truncate(5 * time.Minute)
		closeTime = openTime.Add(5 * time.Minute)
	case "15m":
		openTime = t.Truncate(15 * time.Minute)
		closeTime = openTime.Add(15 * time.Minute)
	case "30m":
		openTime = t.Truncate(30 * time.Minute)
		closeTime = openTime.Add(30 * time.Minute)
	case "1h":
		openTime = t.Truncate(time.Hour)
		closeTime = openTime.Add(time.Hour)
	case "4h":
		openTime = t.Truncate(4 * time.Hour)
		closeTime = openTime.Add(4 * time.Hour)
	case "1d":
		openTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		closeTime = openTime.AddDate(0, 0, 1)
	case "1w":
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		daysToSubtract := weekday - 1
		openTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -daysToSubtract)
		closeTime = openTime.AddDate(0, 0, 7)
	default:
		openTime = t.Truncate(time.Minute)
		closeTime = openTime.Add(time.Minute)
	}

	return openTime, closeTime
}

func (ce *CandleEngine) ProcessTrade(ctx context.Context, symbol string, price, quantity float64, timestamp time.Time) error {
	lockVal, _ := ce.symbolLocks.LoadOrStore(symbol, &sync.Mutex{})
	lock := lockVal.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	price = RoundToPrecision(price, 8)
	quantity = RoundToPrecision(quantity, 8)

	for _, interval := range ce.intervals {
		openTime, closeTime := GetIntervalBoundary(timestamp, interval)

		var c Candle
		var exists bool

		if ce.db != nil {
			err := ce.db.Pool.QueryRow(ctx,
				"SELECT id, open, high, low, close, base_volume, quote_volume, trade_count FROM candles WHERE symbol=$1 AND interval=$2 AND open_time=$3",
				symbol, interval, openTime).Scan(&c.ID, &c.Open, &c.High, &c.Low, &c.Close, &c.BaseVolume, &c.QuoteVolume, &c.TradeCount)
			if err == nil {
				exists = true
			}
		}

		if exists {
			c.Close = price
			if price > c.High {
				c.High = price
			}
			if price < c.Low {
				c.Low = price
			}
			c.BaseVolume = RoundToPrecision(c.BaseVolume+quantity, 8)
			c.QuoteVolume = RoundToPrecision(c.QuoteVolume+(price*quantity), 8)
			c.TradeCount++
			c.CloseTime = closeTime
		} else {
			c = Candle{
				ID:          fmt.Sprintf("cand_%s_%s_%d", symbol, interval, openTime.Unix()),
				Symbol:      symbol,
				Interval:    interval,
				Open:        price,
				High:        price,
				Low:         price,
				Close:       price,
				BaseVolume:  quantity,
				QuoteVolume: RoundToPrecision(price*quantity, 8),
				TradeCount:  1,
				OpenTime:    openTime,
				CloseTime:   closeTime,
			}
		}

		if ce.db != nil {
			if exists {
				_, err := ce.db.Pool.Exec(ctx,
					`UPDATE candles SET high=$1, low=$2, close=$3, base_volume=$4, quote_volume=$5, trade_count=$6, close_time=$7 WHERE id=$8`,
					c.High, c.Low, c.Close, c.BaseVolume, c.QuoteVolume, c.TradeCount, c.CloseTime, c.ID)
				if err != nil {
					return fmt.Errorf("failed to update candle: %w", err)
				}
			} else {
				_, err := ce.db.Pool.Exec(ctx,
					`INSERT INTO candles (id, symbol, interval, open, high, low, close, base_volume, quote_volume, trade_count, open_time, close_time)
					 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
					 ON CONFLICT (symbol, interval, open_time) DO UPDATE SET high=EXCLUDED.high, low=EXCLUDED.low, close=EXCLUDED.close, base_volume=candles.base_volume+EXCLUDED.base_volume, quote_volume=candles.quote_volume+EXCLUDED.quote_volume, trade_count=candles.trade_count+EXCLUDED.trade_count, close_time=EXCLUDED.close_time`,
					c.ID, c.Symbol, c.Interval, c.Open, c.High, c.Low, c.Close, c.BaseVolume, c.QuoteVolume, c.TradeCount, c.OpenTime, c.CloseTime)
				if err != nil {
					return fmt.Errorf("failed to insert candle: %w", err)
				}
			}
		}

		if ce.redisClient != nil {
			key := fmt.Sprintf("candles:%s:%s:latest", symbol, interval)
			data, err := json.Marshal(c)
			if err == nil {
				_ = ce.redisClient.Set(ctx, key, string(data), 24*time.Hour)
			}
		}
	}

	return nil
}

func (ce *CandleEngine) GetCandles(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	if limit <= 0 {
		limit = 100
	}

	var candles []Candle

	if ce.db != nil {
		rows, err := ce.db.Pool.Query(ctx,
			`SELECT id, symbol, interval, open, high, low, close, base_volume, quote_volume, trade_count, open_time, close_time
			 FROM candles WHERE symbol=$1 AND interval=$2 ORDER BY open_time DESC LIMIT $3`,
			symbol, interval, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var c Candle
			err := rows.Scan(&c.ID, &c.Symbol, &c.Interval, &c.Open, &c.High, &c.Low, &c.Close, &c.BaseVolume, &c.QuoteVolume, &c.TradeCount, &c.OpenTime, &c.CloseTime)
			if err == nil {
				candles = append(candles, c)
			}
		}
	}

	return candles, nil
}
