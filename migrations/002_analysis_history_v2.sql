-- =============================================
-- Migration 002: Analysis History V2
-- Jalankan manual di Supabase SQL Editor jika AutoMigrate tidak bekerja
-- =============================================

CREATE TABLE IF NOT EXISTS analysis_history_v2 (
    id           BIGSERIAL PRIMARY KEY,
    symbol       VARCHAR(20)     NOT NULL,
    timeframe    VARCHAR(10)     NOT NULL,
    price        DECIMAL(30, 10) NOT NULL,
    change_24h   DECIMAL(10, 4)  DEFAULT 0,
    volume       DECIMAL(30, 2)  DEFAULT 0,
    signal       VARCHAR(10)     NOT NULL,          -- LONG | SHORT | WAIT
    confidence   INTEGER         DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 100),
    quant_total  INTEGER         DEFAULT 0,          -- quant score 0-100
    quant_signal VARCHAR(10),                        -- LONG | SHORT | WAIT
    ai_model     VARCHAR(30),                        -- claude-sonnet-4-6 | gpt-4o | unavailable
    reasoning    TEXT,
    confluence   VARCHAR(20),                        -- STRONG_LONG | LONG | NEUTRAL | SHORT | STRONG_SHORT
    rsi14        DECIMAL(10, 4),
    ema9         DECIMAL(30, 10),
    ema21        DECIMAL(30, 10),
    atr          DECIMAL(20, 10),
    obv          DECIMAL(30, 4),
    pattern_bias VARCHAR(10),                        -- BULLISH | BEARISH | NEUTRAL
    created_at   TIMESTAMPTZ     DEFAULT NOW(),
    updated_at   TIMESTAMPTZ     DEFAULT NOW()
);

-- Indexes untuk query yang sering
CREATE INDEX IF NOT EXISTS idx_v2_symbol     ON analysis_history_v2(symbol);
CREATE INDEX IF NOT EXISTS idx_v2_created    ON analysis_history_v2(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_v2_signal     ON analysis_history_v2(signal);
CREATE INDEX IF NOT EXISTS idx_v2_sym_time   ON analysis_history_v2(symbol, created_at DESC);

-- Auto-update timestamp
CREATE OR REPLACE FUNCTION update_v2_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_v2_updated_at
    BEFORE UPDATE ON analysis_history_v2
    FOR EACH ROW
    EXECUTE FUNCTION update_v2_updated_at();
