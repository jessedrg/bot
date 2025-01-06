package arbitrage

import (
	"fmt"
	"time"

	"github.com/algorand/go-algorand/data"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/pools"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/protocol"
)

// LiquidityPool contiene los detalles de un pool de liquidez.
type LiquidityPool struct {
    AssetA    basics.AssetIndex
    AssetB    basics.AssetIndex
    PoolSizeA uint64
    PoolSizeB uint64
    Address   basics.Address
    Timestamp int64
    Price     float64
}

// Bot estructura principal del bot de arbitraje
type Bot struct {
    ledger          *data.Ledger
    transactionPool *pools.TransactionPool
    tradingPair     string
    liquidityPools  map[string]*LiquidityPool
    address         basics.Address
}

// NewBot crea una nueva instancia del Bot
func NewBot(ledger *data.Ledger, transactionPool *pools.TransactionPool, tradingPair string, address basics.Address) *Bot {
    return &Bot{
        ledger:          ledger,
        transactionPool: transactionPool,
        tradingPair:     tradingPair,
        liquidityPools:  make(map[string]*LiquidityPool),
        address:         address,
    }
}

// FindLiquidityPools busca los liquidity pools en los últimos bloques.
func (bot *Bot) FindLiquidityPools() ([]LiquidityPool, error) {
    var pools []LiquidityPool

    // Obtener el último bloque del ledger
    latestRound := bot.ledger.Latest()
    startRound := basics.Round(0)
    if latestRound > 1000 {
        startRound = latestRound - 1000
    }

    // Recorrer los bloques
    for round := latestRound; round >= startRound; round-- {
        blk, err := bot.ledger.Block(round)
        if err != nil {
            return nil, fmt.Errorf("error obteniendo bloque %d: %w", round, err)
        }

        // Procesar las transacciones del bloque
        for _, txn := range blk.Payset {
            if pool, ok := bot.processPoolTransaction(txn); ok {
                pools = append(pools, pool)
            }
        }
    }

    return pools, nil
}

// processPoolTransaction procesa transacciones relacionadas con pools de liquidez.
func (bot *Bot) processPoolTransaction(txn transactions.SignedTxnInBlock) (LiquidityPool, bool) {
    // Verificar si es una transacción de aplicación
    if txn.Txn.Type != protocol.ApplicationCallTx {
        return LiquidityPool{}, false
    }

    // Verificar que es una llamada válida a un contrato de liquidez
    if txn.Txn.ApplicationID == 0 {
        return LiquidityPool{}, false
    }

    pool := LiquidityPool{
        AssetA:    txn.Txn.XferAsset,
        AssetB:    basics.AssetIndex(txn.Txn.ApplicationID),
        PoolSizeA: txn.Txn.AssetAmount,
        PoolSizeB: txn.Txn.Amount.Raw,
        Address:   txn.Txn.Sender,
        Timestamp: time.Now().Unix(),
    }

    pool.Price = calculatePoolPrice(pool.PoolSizeA, pool.PoolSizeB)

    return pool, true
}

// calculatePoolPrice calcula el precio de AssetA en términos de AssetB
func calculatePoolPrice(poolSizeA, poolSizeB uint64) float64 {
    if poolSizeB == 0 {
        return 0
    }
    return float64(poolSizeA) / float64(poolSizeB)
}

// FindArbitrageOpportunity identifica oportunidades de arbitraje entre pools
func (bot *Bot) FindArbitrageOpportunity(pools []LiquidityPool) (*LiquidityPool, *LiquidityPool, float64) {
    var bestBuyPool *LiquidityPool
    var bestSellPool *LiquidityPool
    maxProfit := 0.0

    for _, buyPool := range pools {
        for _, sellPool := range pools {
            if buyPool.Address == sellPool.Address {
                continue
            }

            profit := calculateArbitrageProfit(buyPool, sellPool)
            if profit > maxProfit {
                maxProfit = profit
                bestBuyPool = &buyPool
                bestSellPool = &sellPool
            }
        }
    }

    return bestBuyPool, bestSellPool, maxProfit
}

// calculateArbitrageProfit calcula la ganancia potencial
func calculateArbitrageProfit(buyPool, sellPool LiquidityPool) float64 {
    buyPrice := buyPool.Price
    sellPrice := sellPool.Price

    if buyPrice < sellPrice {
        return sellPrice - buyPrice
    }
    return 0.0
}

// ExecuteArbitrage ejecuta la transacción de arbitraje
// ExecuteArbitrage ejecuta la transacción de arbitraje
// ExecuteArbitrage ejecuta la transacción de arbitraje
func (bot *Bot) ExecuteArbitrage(buyPool, sellPool *LiquidityPool) error {
    if buyPool == nil || sellPool == nil {
        return fmt.Errorf("pools inválidos para arbitraje")
    }

    // Obtener la información de genesis del primer bloque
    genesisBlock, err := bot.ledger.Block(basics.Round(0))
    if err != nil {
        return fmt.Errorf("error obteniendo bloque genesis: %w", err)
    }

    // Crear transacción de compra
    buyTxn := transactions.Transaction{
        Type: protocol.ApplicationCallTx,
        Header: transactions.Header{
            Sender:      bot.address,
            FirstValid:  bot.ledger.Latest(),
            LastValid:   bot.ledger.Latest() + 1000,
            Fee:        basics.MicroAlgos{Raw: 1000},
            GenesisID:  genesisBlock.GenesisID(),
            GenesisHash: genesisBlock.GenesisHash(),
        },
        ApplicationCallTxnFields: transactions.ApplicationCallTxnFields{
            ApplicationID: basics.AppIndex(buyPool.AssetB),
            OnCompletion: transactions.NoOpOC,
            ApplicationArgs: [][]byte{},
        },
    }

    // Crear transacción de venta
    sellTxn := transactions.Transaction{
        Type: protocol.ApplicationCallTx,
        Header: transactions.Header{
            Sender:      bot.address,
            FirstValid:  bot.ledger.Latest(),
            LastValid:   bot.ledger.Latest() + 1000,
            Fee:        basics.MicroAlgos{Raw: 1000},
            GenesisID:  genesisBlock.GenesisID(),
            GenesisHash: genesisBlock.GenesisHash(),
        },
        ApplicationCallTxnFields: transactions.ApplicationCallTxnFields{
            ApplicationID: basics.AppIndex(sellPool.AssetB),
            OnCompletion: transactions.NoOpOC,
            ApplicationArgs: [][]byte{},
        },
    }

    // Crear grupo de transacciones
    txGroup := []transactions.SignedTxn{
        {Txn: buyTxn},
        {Txn: sellTxn},
    }

    // Agregar transacciones al pool
    err = bot.transactionPool.Remember(txGroup)
    if err != nil {
        return fmt.Errorf("error al agregar transacciones al pool: %w", err)
    }

    return nil
}