-- =============================================
-- Migration: Create analysis_history table
-- Jalankan di Supabase SQL Editor
-- =============================================

CREATE TABLE IF NOT EXISTS analysis_history (
    id          BIGSERIAL PRIMARY KEY,
    symbol      VARCHAR(20)     NOT NULL,
    timeframe   VARCHAR(10)     NOT NULL,
    price       DECIMAL(30, 10) NOT NULL,
    change_24h  DECIMAL(10, 4)  DEFAULT 0,
    volume      DECIMAL(30, 2)  DEFAULT 0,
    signal      VARCHAR(10)     NOT NULL CHECK (signal IN ('BUY', 'SELL', 'HOLD', 'UNKNOWN')),
    confidence  INTEGER         DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 100),
    reasoning   TEXT,
    -- Indikator teknikal
    rsi14       DECIMAL(10, 4),
    sma20       DECIMAL(30, 10),
    sma50       DECIMAL(30, 10),
    macd        DECIMAL(20, 10),
    macd_signal DECIMAL(20, 10),
    bb_upper    DECIMAL(30, 10),
    bb_lower    DECIMAL(30, 10),
    -- Timestamps
    created_at  TIMESTAMPTZ     DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     DEFAULT NOW()
);

-- Index untuk query yang sering digunakan
CREATE INDEX IF NOT EXISTS idx_analysis_symbol ON analysis_history(symbol);
CREATE INDEX IF NOT EXISTS idx_analysis_created_at ON analysis_history(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analysis_signal ON analysis_history(signal);

-- Index composite untuk history per symbol
CREATE INDEX IF NOT EXISTS idx_analysis_symbol_time ON analysis_history(symbol, created_at DESC);

-- Trigger untuk auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_analysis_updated_at
    BEFORE UPDATE ON analysis_history
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Enable Row Level Security (opsional, untuk keamanan)
-- ALTER TABLE analysis_history ENABLE ROW LEVEL SECURITY;

-- Contoh query yang berguna:
-- SELECT * FROM analysis_history WHERE symbol = 'BTCUSDT' ORDER BY created_at DESC LIMIT 10;
-- SELECT signal, COUNT(*), AVG(confidence) FROM analysis_history GROUP BY signal;
-- SELECT symbol, signal, created_at FROM analysis_history ORDER BY created_at DESC LIMIT 20;
