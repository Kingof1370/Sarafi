# Market Statistics Engine

Velyxora monitors real-time market quality and performance statistics:

- **Spread**: Best Ask - Best Bid
- **Mid Price**: (Best Ask + Best Bid) / 2
- **Weighted Mid Price**: (Best Bid * AskVol + Best Ask * BidVol) / (BidVol + AskVol)
- **Depth Imbalance**: (BidVol - AskVol) / (BidVol + AskVol)
- **Market Health Score**: Evaluated based on narrowness of spreads and depth balance, ranging from 0 to 100.
